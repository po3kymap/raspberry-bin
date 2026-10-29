// Largest side of a photo sent for a check. Phone photos are several MB;
// the model looks at 224 px anyway, so this is plenty and uploads fast.
const MAX_SIDE = 1024

// Turns a camera photo into a JPEG of at most MAX_SIDE px, with the phone's
// rotation applied. This also converts formats the server does not accept,
// such as HEIC from an iPhone gallery.
export async function prepareImage(file) {
  const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
  const scale = Math.min(1, MAX_SIDE / Math.max(bitmap.width, bitmap.height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.round(bitmap.width * scale)
  canvas.height = Math.round(bitmap.height * scale)
  canvas.getContext('2d').drawImage(bitmap, 0, 0, canvas.width, canvas.height)
  bitmap.close()

  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('Could not process the photo.'))), 'image/jpeg', 0.85)
  })
}
