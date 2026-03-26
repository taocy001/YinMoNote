/**
 * Batch encryption/decryption logic for library-wide re-encryption.
 * Converts all notes and assets between plain and encrypted storage.
 */
import { ref, type Ref } from 'vue'
import axios from 'axios'
import * as crypto from '../crypto'

const API_BASE = '/api'

function base64ToBlob(base64: string): Blob {
  const sep = base64.indexOf(';base64,')
  if (sep === -1 || !base64.startsWith('data:')) throw new Error('Invalid data URL')
  const contentType = base64.slice(5, sep)
  const raw = window.atob(base64.slice(sep + 8))
  const uInt8Array = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; ++i) uInt8Array[i] = raw.charCodeAt(i)
  return new Blob([uInt8Array], { type: contentType })
}

export function useBatchEncryption(
  batchResultMsg: Ref<string>,
  lockedLabel: Ref<string>,
  partialFailLabel: Ref<(n: number) => string>,
  onComplete?: () => void
) {
  const batchProcessing = ref(false)
  const batchProgress = ref(0)

  /**
   * Converts all notes and assets between plain and encrypted storage.
   * Assets are overwritten in-place via PUT /api/uploads/:filename so
   * existing URL references in note content remain valid.
   */
  const batchUpdateEncryption = async (toEncrypted: boolean) => {
    if (crypto.isLibraryLocked()) { batchResultMsg.value = lockedLabel.value; return }

    batchProcessing.value = true; batchProgress.value = 0
    let failed = 0
    try {
      const notes: { name: string }[] = (await axios.get(`${API_BASE}/notes`)).data.notes || []
      let assets: string[] = []
      try { assets = (await axios.get(`${API_BASE}/assets`)).data.assets || [] } catch (_) {}

      const total = Math.max(notes.length + assets.length, 1); let processed = 0

      // --- Notes ---
      for (const n of notes) {
        try {
          const res = await axios.get(`${API_BASE}/notes/${n.name}`)
          const raw: string = res.data.content
          const plain = await crypto.decryptText(raw)
          if (plain === '[Decryption Error]' || plain === '[Locked]') { failed++; continue }
          const next = toEncrypted ? await crypto.encryptText(plain) : plain
          if (toEncrypted && !next.startsWith('ENC1:')) { failed++; continue }
          if (next !== raw) await axios.put(`${API_BASE}/notes/${n.name}`, { content: next })
        } catch (_) { failed++ }
        batchProgress.value = Math.round((++processed / total) * 100)
      }

      // --- Assets: overwrite in-place so note URLs stay valid ---
      for (const a of assets) {
        try {
          const res = await axios.get(`/uploads/${a}`, { responseType: 'blob' })
          const blob: Blob = res.data
          const head = await blob.slice(0, 5).text()
          let finalBlob: Blob | null = null

          if (head === 'ENC1:' && !toEncrypted) {
            const plain = await crypto.decryptText(await blob.text())
            if (plain === '[Decryption Error]' || plain === '[Locked]') { failed++; continue }
            finalBlob = plain.startsWith('data:') ? base64ToBlob(plain) : new Blob([plain])
          } else if (head !== 'ENC1:' && toEncrypted) {
            const base64: string = await new Promise((resolve, reject) => {
              const r = new FileReader()
              r.onload = () => resolve(r.result as string)
              r.onerror = reject
              r.readAsDataURL(blob)
            })
            const enc = await crypto.encryptText(base64)
            if (!enc.startsWith('ENC1:')) { failed++; continue }
            finalBlob = new Blob([enc], { type: 'text/plain' })
          }

          if (finalBlob) {
            const fd = new FormData()
            fd.append('image', finalBlob, a)
            await axios.put(`${API_BASE}/uploads/${a}`, fd)
          }
        } catch (_) { failed++ }
        batchProgress.value = Math.round((++processed / total) * 100)
      }

      onComplete?.()

      if (failed > 0) {
        batchResultMsg.value = partialFailLabel.value(failed)
      }
    } finally { batchProcessing.value = false }
  }

  return { batchProcessing, batchProgress, batchUpdateEncryption }
}
