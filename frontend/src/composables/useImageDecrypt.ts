import { ref } from 'vue'
import axios from 'axios'
import * as crypto from '../crypto'

/** Base64-encoded SVG shown when an image is encrypted and the library is locked. */
export const SVG_LOCKED = 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0OCIgaGVpZ2h0PSI0OCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIiBzdHJva2U9ImdyYXkiIHN0cm9rZS13aWR0aD0iMSIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIj48cmVjdCB4PSIzIiB5PSIxMSIgd2lkdGg9IjE4IiBoZWlnaHQ9IjExIiByeD0iMiIgcnk9IjIiPjwvcmVjdD48cGF0aCBkPSJNNyAxMVY3YTUgNSAwIDAgMSAxMCAwdjQiPjwvcGF0aD48L3N2Zz4='

/** Base64-encoded SVG shown when image decryption fails (wrong key or corrupt data). */
export const SVG_DECRYPT_ERROR = 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0OCIgaGVpZ2h0PSI0OCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIiBzdHJva2U9InJlZCIgc3Ryb2tlLXdpZHRoPSIxIiBzdHJva2UtbGluZWNhcD0icm91bmQiIHN0cm9rZS1saW5lam9pbj0icm91bmQiPjxjaXJjbGUgY3g9IjEyIiBjeT0iMTIiIHI9IjEwIj48L2NpcmNsZT48bGluZSB4MT0iMTIiIHkxPSI4IiB4Mj0iMTIiIHkyPSIxMiI+PC9saW5lPjxsaW5lIHgxPSIxMiIgeTE9IjE2IiB4Mj0iMTIuMDEiIHkyPSIxNiI+PC9saW5lPjwvc3ZnPg=='

/**
 * Composable that handles fetching and decrypting an image asset.
 *
 * Fetches the asset from `src`, detects whether it is ENC1-encrypted,
 * and decrypts it when the library is unlocked.
 * Returns `loading` and `displaySrc` refs that the caller can bind to a template.
 */
export function useImageDecrypt() {
  const loading = ref(true)
  const displaySrc = ref('')
  let objectUrl = ''

  const revokeObjectUrl = () => {
    if (objectUrl) {
      URL.revokeObjectURL(objectUrl)
      objectUrl = ''
    }
  }

  /**
   * Fetches and (if necessary) decrypts the image at `src`.
   * @param src                   The asset URL to load.
   * @param isLocked              Whether the library is currently locked.
   * @param allowExternalImages   Whether to load external http/https images (default: false).
   */
  // Guard against concurrent calls: each new call increments the token;
  // when an older call finishes it checks if the token still matches and
  // skips the assignment if a newer call has started.
  let callToken = 0

  const fetchAndDecrypt = async (src: string, isLocked: boolean, allowExternalImages = false): Promise<void> => {
    if (!src) return
    const myToken = ++callToken

    revokeObjectUrl()

    if (src.startsWith('data:')) {
      displaySrc.value = src
      loading.value = false
      return
    }

    // SEC-016: Block external images unless explicitly allowed to prevent IP leakage.
    if (src.startsWith('http')) {
      if (allowExternalImages) {
        displaySrc.value = src
      } else {
        displaySrc.value = SVG_LOCKED
      }
      loading.value = false
      return
    }

    try {
      loading.value = true
      const res = await axios.get(src, { responseType: 'blob' })
      const blob = res.data
      const head = await blob.slice(0, 5).text()

      if (head === 'ENC1:') {
        if (isLocked) {
          displaySrc.value = SVG_LOCKED
          loading.value = false
          return
        }
        const textData = await blob.text()
        // Yield to the event loop to avoid blocking the UI thread during decryption.
        await new Promise(resolve => setTimeout(resolve, 0))
        const decryptedBase64 = await crypto.decryptText(textData)
        // decryptText returns '[Decryption Error]' on failure, '[Locked]' if the
        // library was locked mid-request, or an 'ENC1:' prefix if decryption
        // produced another ciphertext (wrong key / corrupted data).
        const isFail = decryptedBase64 === '[Decryption Error]'
          || decryptedBase64 === '[Locked]'
          || decryptedBase64.startsWith('ENC1:')
        if (myToken !== callToken) return // superseded by a newer call
        displaySrc.value = isFail ? SVG_DECRYPT_ERROR : decryptedBase64
      } else {
        if (myToken !== callToken) return // superseded by a newer call
        objectUrl = URL.createObjectURL(blob)
        displaySrc.value = objectUrl
      }
    } catch {
      if (myToken !== callToken) return // superseded by a newer call
      displaySrc.value = SVG_DECRYPT_ERROR
    } finally {
      loading.value = false
    }
  }

  const cleanup = () => { revokeObjectUrl(); displaySrc.value = '' }

  return { loading, displaySrc, fetchAndDecrypt, cleanup }
}
