#!/usr/bin/env python3
"""Raspberry Bin client that runs on the Raspberry Pi inside the bin.

It waits for commands from the server (long polling) and:
  - capture:   takes a photo, sends it to the server and opens the lid
               for the returned category if the item can be sorted;
  - open_lid / close_lid: opens or closes a lid (admin, FR4).

The Pi only makes outgoing HTTPS requests to our server; it opens no ports.
Written for Raspberry Pi OS (Raspbian) Buster: Python 3.7 and the
"picamera" library.

Settings come from environment variables (see raspberry-bin.env.example).
"""

import io
import logging
import os
import signal
import sys
import time

import requests

COMPARTMENTS = ("GENERAL_WASTE", "ORGANIC", "PAPER", "PMD")

# Long polling: the server answers within WAIT_SECONDS; the HTTP timeout is a
# bit longer so a slow network does not cut the request off.
WAIT_SECONDS = 25
POLL_TIMEOUT = WAIT_SECONDS + 10
UPLOAD_TIMEOUT = 45

log = logging.getLogger("raspberry-bin")


def setting(name, default=None):
    value = os.environ.get(name, default)
    if value is None or value == "":
        sys.exit("Missing setting {}; see raspberry-bin.env.example".format(name))
    return value


class Camera:
    """Takes JPEG photos with the Pi camera. The camera stays open between
    scans, so a photo takes well under a second."""

    def __init__(self, width, height, rotation):
        from picamera import PiCamera  # only available on the Pi

        self.camera = PiCamera(resolution=(width, height))
        self.camera.rotation = rotation
        time.sleep(2)  # let exposure and white balance settle

    def capture(self):
        stream = io.BytesIO()
        self.camera.capture(stream, format="jpeg", quality=85)
        return stream.getvalue()


class FileCamera:
    """Stand-in for the camera when testing without one: always returns the
    same photo from disk (setting RBIN_TEST_PHOTO)."""

    def __init__(self, path):
        with open(path, "rb") as f:
            self.photo = f.read()

    def capture(self):
        return self.photo


class Lids:
    """Opens and closes the compartment lids.

    The hardware (servos and solenoid locks) is not connected yet, so this
    only logs what it would do. When the wiring is known, replace the bodies
    of open() and close() with gpiozero code for the pins of each compartment.
    """

    def open(self, compartment):
        log.info("LID %s: open", compartment)

    def close(self, compartment):
        log.info("LID %s: close", compartment)


class BinClient:
    def __init__(self, server, bin_id, device_key, camera, lids, open_seconds):
        self.server = server.rstrip("/")
        self.bin_id = bin_id
        self.camera = camera
        self.lids = lids
        self.open_seconds = open_seconds

        self.http = requests.Session()
        self.http.headers["X-API-Key"] = device_key
        self.http.headers["User-Agent"] = "raspberry-bin-pi/1.0"

        self.running = True

    def run(self):
        log.info("Waiting for commands for %s at %s", self.bin_id, self.server)
        backoff = 1

        while self.running:
            try:
                response = self.http.get(
                    self.server + "/api/device/commands",
                    params={"bin_id": self.bin_id, "wait": WAIT_SECONDS},
                    timeout=POLL_TIMEOUT,
                )
            except requests.RequestException as e:
                # No network or server down: wait longer each time, up to 30 s
                log.warning("Server not reachable (%s), retrying in %d s", e.__class__.__name__, backoff)
                time.sleep(backoff)
                backoff = min(backoff * 2, 30)
                continue

            backoff = 1
            if response.status_code == 204:
                continue  # nothing to do, ask again
            if response.status_code == 401:
                log.error("Server refused the device key; check RBIN_DEVICE_KEY")
                time.sleep(30)
                continue
            if response.status_code != 200:
                log.error("Unexpected answer %d: %s", response.status_code, response.text[:200])
                time.sleep(5)
                continue

            try:
                self.handle(response.json())
            except Exception:
                # A failing command must never stop the bin
                log.exception("Command failed")

    def handle(self, command):
        name = command.get("command")
        log.info("Command: %s", name)

        if name == "capture":
            self.scan(command.get("request_id", ""))
        elif name in ("open_lid", "close_lid"):
            compartment = command.get("compartment")
            if compartment not in COMPARTMENTS:
                log.error("Unknown compartment %r", compartment)
                return
            if name == "open_lid":
                self.lids.open(compartment)
            else:
                self.lids.close(compartment)
        else:
            log.warning("Unknown command %r", name)

    def scan(self, request_id):
        started = time.monotonic()
        photo = self.camera.capture()
        log.info("Photo taken (%d KB), sending", len(photo) // 1024)

        response = self.http.post(
            self.server + "/api/scan",
            data={"bin_id": self.bin_id, "request_id": request_id},
            files={"image": ("scan.jpg", photo, "image/jpeg")},
            timeout=UPLOAD_TIMEOUT,
        )
        if response.status_code != 200:
            log.error("Scan refused %d: %s", response.status_code, response.text[:200])
            return

        result = response.json()
        log.info(
            "Result: %s (%.0f%%), can_sort=%s %s in %.1f s",
            result.get("category"),
            100 * result.get("confidence", 0),
            result.get("can_sort"),
            result.get("reason") or "",
            time.monotonic() - started,
        )

        # Only open a lid when the server says the item can be sorted (FR2, FR6)
        category = result.get("category")
        if result.get("can_sort") and category in COMPARTMENTS:
            self.lids.open(category)
            time.sleep(self.open_seconds)  # FR3: close after a fixed delay
            self.lids.close(category)

    def stop(self, *_):
        log.info("Stopping")
        self.running = False


def main():
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    server = setting("RBIN_SERVER_URL", "https://raspberry-bin-27.swedencentral.cloudapp.azure.com")
    if not server.startswith("https://"):
        sys.exit("RBIN_SERVER_URL must use https://")

    test_photo = os.environ.get("RBIN_TEST_PHOTO")
    if test_photo:
        camera = FileCamera(test_photo)
        log.info("Using test photo %s instead of the camera", test_photo)
    else:
        camera = Camera(
            int(setting("RBIN_PHOTO_WIDTH", "1024")),
            int(setting("RBIN_PHOTO_HEIGHT", "768")),
            int(setting("RBIN_CAMERA_ROTATION", "0")),
        )

    client = BinClient(
        server=server,
        bin_id=setting("RBIN_BIN_ID", "bin-27"),
        device_key=setting("RBIN_DEVICE_KEY"),
        camera=camera,
        lids=Lids(),
        open_seconds=float(setting("RBIN_LID_OPEN_SECONDS", "5")),
    )

    # systemd stops the service with SIGTERM: finish the current command first
    signal.signal(signal.SIGTERM, client.stop)
    try:
        client.run()
    except KeyboardInterrupt:  # Ctrl+C when started by hand
        log.info("Stopped")


if __name__ == "__main__":
    main()
