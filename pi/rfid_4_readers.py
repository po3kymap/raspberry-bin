#!/usr/bin/env python3
"""
4 x RC522 (RFID) на одном Raspberry Pi, у каждого ридера своя карта.

ПОДКЛЮЧЕНИЕ (физические номера пинов на гребёнке Pi):
    3.3V  -> пин 1        (НЕ 5V!)
    GND   -> пин 6
    MOSI  -> пин 19 (GPIO10)
    MISO  -> пин 21 (GPIO9)
    SCK   -> пин 23 (GPIO11)
    RST   -> пин 22 (GPIO25)   общий для всех ридеров
    SDA ридера 1 -> пин 29 (GPIO5)
    SDA ридера 2 -> пин 31 (GPIO6)
    SDA ридера 3 -> пин 33 (GPIO13)
    SDA ридера 4 -> пин 35 (GPIO19)
    IRQ   -> не подключается

    SDA сидят на обычных GPIO, программа сама выбирает ридер (ручной chip select).
    Пины 24 и 26 (CE0/CE1) не используются: аппаратный SPI дёргает их сам,
    там ничего не подключено, поэтому ридеры не мешают друг другу.

ПОДГОТОВКА PI (один раз):
    1) sudo raspi-config -> Interface Options -> SPI -> Enable, потом перезагрузка
    2) sudo apt install -y python3-spidev python3-gpiozero python3-lgpio
       (на Raspberry Pi OS обычно уже стоят)

ЗАПУСК:
    python3 rfid_4_readers.py

Сначала запусти и приложи каждую карту к своему ридеру: программа напечатает UID.
Впиши эти UID в EXPECTED_UIDS ниже, тогда она будет писать «правильная / чужая карта».
Работает на Pi 3 / 4 / 5.
"""

import time

import spidev
from gpiozero import DigitalOutputDevice

# ---------------- настройки ----------------
SDA_GPIO = [5, 6, 13, 19]      # BCM-номера GPIO для SDA ридеров 1..4 (пины 29, 31, 33, 35)
RST_GPIO = 25                  # пин 22
SPI_BUS, SPI_DEV = 0, 0
SPI_SPEED_HZ = 1_000_000

# UID карты, которая "принадлежит" каждому ридеру (впиши после первого запуска).
# Формат: "A1B2C3D4". None = не проверять.
EXPECTED_UIDS = [None, None, None, None]

POLL_PAUSE_S = 0.05            # пауза между кругами опроса
MISSES_TO_REMOVE = 3           # сколько пустых опросов подряд = карту убрали
# -------------------------------------------

# Регистры и команды MFRC522
CommandReg, ComIEnReg, ComIrqReg, ErrorReg = 0x01, 0x02, 0x04, 0x06
FIFODataReg, FIFOLevelReg, ControlReg, BitFramingReg = 0x09, 0x0A, 0x0C, 0x0D
ModeReg, TxControlReg, TxASKReg = 0x11, 0x14, 0x15
TModeReg, TPrescalerReg, TReloadRegH, TReloadRegL = 0x2A, 0x2B, 0x2C, 0x2D
VersionReg = 0x37
CMD_IDLE, CMD_TRANSCEIVE, CMD_SOFTRESET = 0x00, 0x0C, 0x0F
PICC_REQA, PICC_ANTICOLL_CL1 = 0x26, 0x93


class RC522:
    def __init__(self, spi, sda_gpio, name):
        self.spi = spi
        self.cs = DigitalOutputDevice(sda_gpio, initial_value=True)  # HIGH = не выбран
        self.name = name

    # --- низкий уровень ---
    def _xfer(self, data):
        self.cs.off()                      # SDA LOW -> говорим с этим ридером
        try:
            return self.spi.xfer2(data)
        finally:
            self.cs.on()                   # SDA HIGH -> отпускаем

    def write(self, reg, val):
        self._xfer([(reg << 1) & 0x7E, val & 0xFF])

    def read(self, reg):
        return self._xfer([((reg << 1) & 0x7E) | 0x80, 0])[1]

    def set_bits(self, reg, mask):
        self.write(reg, self.read(reg) | mask)

    def clear_bits(self, reg, mask):
        self.write(reg, self.read(reg) & ~mask)

    # --- инициализация ---
    def init(self):
        self.write(CommandReg, CMD_SOFTRESET)
        time.sleep(0.05)
        self.write(TModeReg, 0x8D)
        self.write(TPrescalerReg, 0x3E)
        self.write(TReloadRegL, 30)
        self.write(TReloadRegH, 0)
        self.write(TxASKReg, 0x40)
        self.write(ModeReg, 0x3D)
        self.set_bits(TxControlReg, 0x03)  # антенна вкл
        return self.read(VersionReg)

    # --- обмен с картой ---
    def _transceive(self, data):
        self.write(ComIEnReg, 0x77 | 0x80)
        self.write(ComIrqReg, 0x7F)        # сбросить флаги прерываний
        self.set_bits(FIFOLevelReg, 0x80)  # очистить FIFO
        self.write(CommandReg, CMD_IDLE)
        for b in data:
            self.write(FIFODataReg, b)
        self.write(CommandReg, CMD_TRANSCEIVE)
        self.set_bits(BitFramingReg, 0x80) # StartSend

        irq = 0
        for _ in range(200):
            irq = self.read(ComIrqReg)
            if irq & 0x31:                 # RxIRq | IdleIRq | TimerIRq
                break
        self.clear_bits(BitFramingReg, 0x80)

        if not (irq & 0x30) or (irq & 0x01) and not (irq & 0x20):
            return None, 0                 # таймаут: карты нет
        if self.read(ErrorReg) & 0x1B:
            return None, 0                 # ошибка/коллизия

        n = self.read(FIFOLevelReg)
        last_bits = self.read(ControlReg) & 0x07
        bits = (n - 1) * 8 + last_bits if last_bits else n * 8
        back = [self.read(FIFODataReg) for _ in range(min(n, 16))]
        return back, bits

    def _request(self):
        self.write(BitFramingReg, 0x07)    # 7 бит
        back, bits = self._transceive([PICC_REQA])
        return back is not None and bits == 0x10

    def _anticoll(self):
        self.write(BitFramingReg, 0x00)
        back, _ = self._transceive([PICC_ANTICOLL_CL1, 0x20])
        if not back or len(back) != 5:
            return None
        if back[0] ^ back[1] ^ back[2] ^ back[3] != back[4]:
            return None
        return back[:4]

    def read_uid(self):
        """UID карты строкой 'A1B2C3D4' или None, если карты нет."""
        # После прошлого чтения карта может "проспать" один запрос — пробуем дважды.
        if not (self._request() or self._request()):
            return None
        uid = self._anticoll()
        return "".join(f"{b:02X}" for b in uid) if uid else None


def main():
    rst = DigitalOutputDevice(RST_GPIO, initial_value=False)
    time.sleep(0.05)
    rst.on()                               # выход из аппаратного сброса
    time.sleep(0.05)

    spi = spidev.SpiDev()
    spi.open(SPI_BUS, SPI_DEV)
    spi.max_speed_hz = SPI_SPEED_HZ
    spi.mode = 0

    readers = [RC522(spi, g, f"Ридер {i + 1}") for i, g in enumerate(SDA_GPIO)]

    print("Проверка ридеров:")
    for r in readers:
        v = r.init()
        status = "OK" if v not in (0x00, 0xFF) else "НЕ ОТВЕЧАЕТ — проверь провода (особенно SDA и MISO)"
        print(f"  {r.name}: версия чипа 0x{v:02X} — {status}")
    print("\nПрикладывай карты (Ctrl+C — выход)\n")

    present = [None] * len(readers)
    misses = [0] * len(readers)

    try:
        while True:
            for i, r in enumerate(readers):
                uid = r.read_uid()
                if uid:
                    misses[i] = 0
                    if uid != present[i]:
                        present[i] = uid
                        exp = EXPECTED_UIDS[i]
                        verdict = "" if exp is None else ("  -> правильная карта" if uid == exp else "  -> ЧУЖАЯ карта")
                        print(f"{r.name}: карта {uid}{verdict}")
                elif present[i] is not None:
                    misses[i] += 1
                    if misses[i] >= MISSES_TO_REMOVE:
                        print(f"{r.name}: карту убрали")
                        present[i] = None
                        misses[i] = 0
            time.sleep(POLL_PAUSE_S)
    except KeyboardInterrupt:
        print("\nВыход")
    finally:
        spi.close()


if __name__ == "__main__":
    main()
