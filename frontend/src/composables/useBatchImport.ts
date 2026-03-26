/**
 * Composable for batch-importing notes from files, folders, or ZIP archives.
 */
import { ref } from 'vue'
import axios from 'axios'
import { unzipSync } from 'fflate'
import { generateId } from './useLibrary'
import * as crypto from '../crypto'

const API_BASE = '/api'

export interface ImportResult {
  fileName: string
  status: 'success' | 'skipped' | 'failed'
  reason?: string
}

export interface ParsedEntry {
  relativePath: string
  content: string | Uint8Array
  isAsset: boolean
}

const VALID_NOTE_EXTS = /\.(md|txt)$/i
const VALID_IMAGE_EXTS = /\.(png|jpg|jpeg|gif|webp)$/i

const MIME_MAP: Record<string, string> = {
  '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg',
  '.gif': 'image/gif', '.webp': 'image/webp',
}

// ── Helpers ─────────────────────────────────────────────────────────────────

export function titleFromPath(relativePath: string): string {
  const name = relativePath.split('/').pop() || relativePath
  const dotIdx = name.lastIndexOf('.')
  return dotIdx > 0 ? name.slice(0, dotIdx) : name
}

export function dirParts(relativePath: string): string[] {
  const segments = relativePath.split('/')
  segments.pop()
  return segments.filter(Boolean)
}

/** Strip common path prefix shared by all entries (e.g. ZIP root folder). */
export function stripCommonPrefix(entries: ParsedEntry[]): ParsedEntry[] {
  if (entries.length === 0) return entries
  const paths = entries.map(e => e.relativePath)
  const first = paths[0].split('/')
  let prefixLen = 0
  for (let i = 0; i < first.length - 1; i++) {
    const seg = first[i]
    if (paths.every(p => p.split('/')[i] === seg)) prefixLen = i + 1
    else break
  }
  if (prefixLen === 0) return entries
  const prefix = first.slice(0, prefixLen).join('/') + '/'
  return entries.map(e => ({ ...e, relativePath: e.relativePath.slice(prefix.length) }))
}

/** Resolve a relative image path from the perspective of a markdown file. */
export function resolveRelativePath(mdPath: string, imgRef: string): string {
  // imgRef like "../assets/img.png" relative to "docs/chapter1/note.md"
  const mdDir = mdPath.split('/').slice(0, -1)
  const imgParts = imgRef.split('/')
  const resolved = [...mdDir]
  for (const p of imgParts) {
    if (p === '..') resolved.pop()
    else if (p !== '.' && p !== '') resolved.push(p)
  }
  return resolved.join('/')
}

/** Rewrite image refs in markdown, resolving relative to the md file's location. */
export function rewriteImageRefs(md: string, mdPath: string, assetUrlMap: Map<string, string>): string {
  let result = md.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_match, alt, ref) => {
    if (ref.startsWith('http://') || ref.startsWith('https://') || ref.startsWith('data:')) return _match
    // Try exact match first, then resolved path, then just filename
    const resolved = assetUrlMap.get(ref)
      || assetUrlMap.get(resolveRelativePath(mdPath, ref))
      || assetUrlMap.get(ref.split('/').pop()!)
    return resolved ? `![${alt}](${resolved})` : _match
  })
  result = result.replace(/<img([^>]*)\ssrc=["']([^"']+)["']/gi, (_match, attrs, ref) => {
    if (ref.startsWith('http://') || ref.startsWith('https://') || ref.startsWith('data:')) return _match
    const resolved = assetUrlMap.get(ref)
      || assetUrlMap.get(resolveRelativePath(mdPath, ref))
      || assetUrlMap.get(ref.split('/').pop()!)
    return resolved ? `<img${attrs} src="${resolved}"` : _match
  })
  return result
}

// ── ZIP parsing ─────────────────────────────────────────────────────────────

function parseZip(buffer: ArrayBuffer): ParsedEntry[] {
  const entries: ParsedEntry[] = []
  const unzipped = unzipSync(new Uint8Array(buffer), {
    filter: (file) => {
      const name = file.name
      if (name.endsWith('/') || name.includes('__MACOSX') || name.includes('.DS_Store')) return false
      return VALID_NOTE_EXTS.test(name) || VALID_IMAGE_EXTS.test(name)
    }
  })
  for (const [name, data] of Object.entries(unzipped)) {
    const isAsset = VALID_IMAGE_EXTS.test(name)
    entries.push({
      relativePath: name,
      content: isAsset ? data : new TextDecoder().decode(data),
      isAsset,
    })
  }
  return stripCommonPrefix(entries)
}

// ── Folder traversal (drag-drop) ────────────────────────────────────────────

async function traverseEntry(entry: FileSystemEntry): Promise<File[]> {
  if (entry.isFile) {
    return new Promise((resolve) => {
      (entry as FileSystemFileEntry).file(f => resolve([f]), () => resolve([]))
    })
  }
  const dirReader = (entry as FileSystemDirectoryEntry).createReader()
  const files: File[] = []
  const readBatch = (): Promise<void> => new Promise((resolve) => {
    dirReader.readEntries(async (entries) => {
      if (entries.length === 0) { resolve(); return }
      for (const e of entries) files.push(...await traverseEntry(e))
      await readBatch()
      resolve()
    }, () => resolve())
  })
  await readBatch()
  return files
}

export async function filesFromDrop(dataTransfer: DataTransfer): Promise<File[]> {
  const items = Array.from(dataTransfer.items)
  const hasEntries = items.some(i => i.webkitGetAsEntry?.()?.isDirectory)
  if (hasEntries) {
    const files: File[] = []
    for (const item of items) {
      const entry = item.webkitGetAsEntry()
      if (entry) files.push(...await traverseEntry(entry))
    }
    return files
  }
  return Array.from(dataTransfer.files)
}

// ── Core import logic ───────────────────────────────────────────────────────

export function useBatchImport(deps: {
  structure: { value: any }
  noteTitles: Record<string, string>
  saveStructure: () => Promise<void>
  indexNote: (id: string, text: string) => void
}) {
  const importing = ref(false)
  const importProgress = ref(0)
  const importStatus = ref('')
  const importResults = ref<ImportResult[]>([])
  const showImportResults = ref(false)

  async function uploadAsset(data: Uint8Array, filename: string): Promise<string | null> {
    try {
      const ext = (filename.match(/\.(png|jpg|jpeg|gif|webp)$/i)?.[0] || '.png').toLowerCase()
      const mime = MIME_MAP[ext] || 'application/octet-stream'
      const safeName = `import${ext}`

      let content: Blob
      if (crypto.isServerEncryptEnabled() && !crypto.isLibraryLocked()) {
        const base64 = await new Promise<string>((resolve, reject) => {
          const r = new FileReader()
          r.onload = () => resolve(r.result as string)
          r.onerror = reject
          r.readAsDataURL(new Blob([data], { type: mime }))
        })
        const encrypted = await crypto.encryptText(base64)
        content = new Blob([encrypted], { type: 'text/plain' })
      } else {
        content = new Blob([data], { type: mime })
      }

      const fd = new FormData()
      fd.append('image', content, safeName)
      const res = await axios.post(`${API_BASE}/upload`, fd)
      return res.data.markdown_url || res.data.preview_url
    } catch (err) {
      console.warn('[YinMo] Asset upload failed:', filename, err)
      return null
    }
  }

  async function importNotes(source: File[] | ArrayBuffer): Promise<void> {
    importing.value = true
    importProgress.value = 0
    importStatus.value = ''
    importResults.value = []

    try {
      // ── Parse entries ─────────────────────────────────────────────────
      let entries: ParsedEntry[]

      if (source instanceof ArrayBuffer) {
        entries = parseZip(source)
      } else {
        entries = []
        for (const file of source) {
          const isAsset = VALID_IMAGE_EXTS.test(file.name)
          const isNote = VALID_NOTE_EXTS.test(file.name)
          if (!isAsset && !isNote) continue
          const relativePath = (file as any).webkitRelativePath || file.name
          if (relativePath.includes('__MACOSX') || relativePath.includes('.DS_Store')) continue
          if (isAsset) {
            entries.push({ relativePath, content: new Uint8Array(await file.arrayBuffer()), isAsset: true })
          } else {
            entries.push({ relativePath, content: await file.text(), isAsset: false })
          }
        }
        // Strip common prefix for folder imports (webkitdirectory always has root folder)
        if (entries.length > 0 && entries.some(e => e.relativePath.includes('/'))) {
          entries = stripCommonPrefix(entries)
        }
      }

      const noteEntries = entries.filter(e => !e.isAsset)
      const assetEntries = entries.filter(e => e.isAsset)

      if (noteEntries.length === 0) { importing.value = false; return }

      // ── Fetch config ───────────────────────────────────────────────────
      let maxNoteSize = 512 * 1024
      let maxNestingDepth = 3
      try {
        const cfgRes = await axios.get(`${API_BASE}/config`)
        maxNoteSize = cfgRes.data.maxNoteSize || maxNoteSize
        maxNestingDepth = cfgRes.data.maxNestingDepth || maxNestingDepth
      } catch { /* defaults */ }

      const total = noteEntries.length + assetEntries.length
      let processed = 0
      const results: ImportResult[] = []
      const newNoteIds: string[] = []

      // ── Upload assets → build path mapping ─────────────────────────────
      // Key: asset's relativePath (after prefix stripping), Value: uploaded URL
      const assetUrlMap = new Map<string, string>()

      for (const asset of assetEntries) {
        importStatus.value = asset.relativePath.split('/').pop() || ''
        const url = await uploadAsset(asset.content as Uint8Array, asset.relativePath)
        if (url) {
          assetUrlMap.set(asset.relativePath, url)
          // Also map by filename only for simple references
          assetUrlMap.set(asset.relativePath.split('/').pop()!, url)
        }
        processed++
        importProgress.value = Math.round((processed / total) * 100)
      }

      // ── Build folder hierarchy ─────────────────────────────────────────
      const folderNoteIds = new Map<string, string>()
      const hasSubdirs = noteEntries.some(e => e.relativePath.includes('/'))

      if (hasSubdirs) {
        const allDirs = new Set<string>()
        for (const entry of noteEntries) {
          const parts = dirParts(entry.relativePath)
          for (let i = 1; i <= parts.length; i++) allDirs.add(parts.slice(0, i).join('/'))
        }
        const sortedDirs = Array.from(allDirs).sort((a, b) => a.split('/').length - b.split('/').length)

        for (const dirPath of sortedDirs) {
          const depth = dirPath.split('/').length
          // Server counts depth from 1 (order=1, child=2, grandchild=3).
          // A folder at path depth N occupies server depth N+1 (it's a child of
          // the order entry). Skip folders that would exceed the server limit.
          if (depth >= maxNestingDepth) continue

          const id = generateId()
          const dirName = dirPath.split('/').pop()!
          folderNoteIds.set(dirPath, id)

          let content = `# ${dirName}\n`
          if (crypto.isServerEncryptEnabled() && !crypto.isLibraryLocked()) {
            content = await crypto.encryptText(content)
          }

          try {
            await axios.put(`${API_BASE}/notes/${id}`, { content })
            deps.noteTitles[id] = dirName
            newNoteIds.push(id)

            const parentDirPath = dirPath.split('/').slice(0, -1).join('/')
            const parentId = parentDirPath ? folderNoteIds.get(parentDirPath) : undefined
            if (parentId) {
              deps.structure.value.parents[id] = parentId
              if (!deps.structure.value.childOrder[parentId]) deps.structure.value.childOrder[parentId] = []
              deps.structure.value.childOrder[parentId].push(id)
            }
          } catch (err) {
            console.warn('[YinMo] Folder note creation failed:', dirPath, err)
          }
        }
      }

      // ── Upload notes ───────────────────────────────────────────────────
      for (const entry of noteEntries) {
        const fileName = entry.relativePath.split('/').pop() || entry.relativePath
        importStatus.value = fileName

        let content = entry.content as string

        // Rewrite image references relative to this md file's location
        if (assetUrlMap.size > 0) {
          content = rewriteImageRefs(content, entry.relativePath, assetUrlMap)
        }

        const plainText = content

        if (crypto.isServerEncryptEnabled() && !crypto.isLibraryLocked()) {
          content = await crypto.encryptText(content)
        }

        if (content.length > maxNoteSize) {
          results.push({ fileName, status: 'skipped', reason: 'too_large' })
          processed++
          importProgress.value = Math.round((processed / total) * 100)
          continue
        }

        const id = generateId()
        try {
          await axios.put(`${API_BASE}/notes/${id}`, { content })
          deps.noteTitles[id] = titleFromPath(entry.relativePath)
          deps.indexNote(id, plainText)
          newNoteIds.push(id)

          const parts = dirParts(entry.relativePath)
          if (parts.length > 0) {
            let parentId: string | undefined
            for (let i = Math.min(parts.length, maxNestingDepth - 1); i > 0; i--) {
              parentId = folderNoteIds.get(parts.slice(0, i).join('/'))
              if (parentId) break
            }
            if (parentId) {
              deps.structure.value.parents[id] = parentId
              if (!deps.structure.value.childOrder[parentId]) deps.structure.value.childOrder[parentId] = []
              deps.structure.value.childOrder[parentId].push(id)
            }
          }

          results.push({ fileName, status: 'success' })
        } catch (err: any) {
          const errCode = err?.response?.data?.error
          if (errCode === 'limit_total_notes') {
            results.push({ fileName, status: 'skipped', reason: 'quota_full' })
            for (let j = noteEntries.indexOf(entry) + 1; j < noteEntries.length; j++) {
              results.push({ fileName: noteEntries[j].relativePath.split('/').pop() || '', status: 'skipped', reason: 'quota_full' })
            }
            break
          }
          results.push({ fileName, status: 'failed', reason: errCode || 'server_error' })
        }

        processed++
        importProgress.value = Math.round((processed / total) * 100)
      }

      // ── Update structure ───────────────────────────────────────────────
      if (newNoteIds.length > 0) {
        for (const id of newNoteIds) {
          if (!deps.structure.value.parents[id]) {
            deps.structure.value.order.unshift(id)
          }
        }
        await deps.saveStructure()
      }

      importResults.value = results
      showImportResults.value = true
    } finally {
      importing.value = false
      importProgress.value = 100
    }
  }

  return {
    importing, importProgress, importStatus, importResults, showImportResults,
    importNotes, filesFromDrop,
  }
}
