<template>
  <Teleport to="body">
    <Transition name="settings-overlay">
      <div v-if="modelValue" class="fixed inset-0 z-[200]" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" @click.self="emit('close')"></div>
    </Transition>
    <Transition name="settings-panel">
      <div v-if="modelValue" data-testid="settings-panel" class="fixed top-0 right-0 bottom-0 z-[201] flex flex-col" style="width: min(480px, 100vw); background: var(--bg-editor); border-left: 1px solid var(--border); box-shadow: var(--shadow-lg);">
        <!-- Panel header -->
        <div class="flex items-center justify-between px-6 py-4 shrink-0" style="border-bottom: 1px solid var(--border);">
          <span class="font-bold text-base" style="color: var(--text-primary);">{{ t.settings }}</span>
          <button data-testid="settings-close-btn" @click="emit('close')" class="sidebar-btn w-7 h-7 flex items-center justify-center rounded-lg transition-all" style="color: var(--text-muted);">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
          </button>
        </div>
        <!-- Tab bar -->
        <div class="flex px-6 pt-3 gap-1 shrink-0" style="border-bottom: 1px solid var(--border);">
          <button v-for="tab in (['appearance', 'editor', 'security', 'ai'] as const)" :key="tab"
            :data-testid="'tab-' + tab"
            @click="emit('update:settingsTab', tab)"
            class="px-3 py-2 ts-sm font-medium rounded-t-lg transition-all focus-ring"
            :style="settingsTab === tab ? 'color: var(--accent); border-bottom: 2px solid var(--accent); margin-bottom: -1px;' : 'color: var(--text-muted);'">
            {{ tab === 'appearance' ? t.appearance : tab === 'security' ? t.security : tab === 'editor' ? t.editor : t.ai }}
          </button>
        </div>
        <!-- Scrollable content -->
        <div class="flex-1 overflow-y-auto px-6 py-4 space-y-3">
          <!-- Appearance tab -->
          <template v-if="settingsTab === 'appearance'">
            <div class="flex items-center justify-between px-4 py-3 rounded-xl" style="background: var(--bg-app); border: 1px solid var(--border);">
              <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.toggleTheme }}</span>
              <div class="flex p-1 rounded-lg gap-1" style="background: var(--bg-hover);">
                <button data-testid="theme-light-btn" @click="emit('update:draftSettings', { ...draftSettings, themeMode: 'light' })" class="px-3 py-1 text-xs rounded-md transition-all font-medium" :style="draftSettings.themeMode === 'light' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">{{ t.themeLight }}</button>
                <button data-testid="theme-auto-btn" @click="emit('update:draftSettings', { ...draftSettings, themeMode: 'auto' })" class="px-3 py-1 text-xs rounded-md transition-all font-medium" :style="draftSettings.themeMode === 'auto' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">{{ t.themeAuto }}</button>
                <button data-testid="theme-dark-btn" @click="emit('update:draftSettings', { ...draftSettings, themeMode: 'dark' })" class="px-3 py-1 text-xs rounded-md transition-all font-medium" :style="draftSettings.themeMode === 'dark' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">{{ t.themeDark }}</button>
              </div>
            </div>
            <div class="flex items-center justify-between px-4 py-3 rounded-xl" style="background: var(--bg-app); border: 1px solid var(--border);">
              <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.language }}</span>
              <div class="flex p-1 rounded-lg gap-1" style="background: var(--bg-hover);">
                <button data-testid="lang-zh-btn" @click="emit('update:draftSettings', { ...draftSettings, lang: 'zh' })" class="px-3 py-1 text-xs rounded-md transition-all font-medium" :style="draftSettings.lang === 'zh' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">中文</button>
                <button data-testid="lang-en-btn" @click="emit('update:draftSettings', { ...draftSettings, lang: 'en' })" class="px-3 py-1 text-xs rounded-md transition-all font-medium" :style="draftSettings.lang === 'en' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">English</button>
              </div>
            </div>
          </template>

          <!-- Editor tab -->
          <template v-if="settingsTab === 'editor'">
            <div class="px-4 py-3 rounded-xl space-y-2" style="background: var(--bg-app); border: 1px solid var(--border);">
              <label class="text-sm font-bold" style="color: var(--text-primary);">{{ t.editorMaxWidth }}</label>
              <div class="flex p-1 rounded-lg gap-1" style="background: var(--bg-hover);">
                <button @click="emit('update:draftSettings', { ...draftSettings, editorWidth: 'standard' })" class="flex-1 py-1.5 text-xs rounded-md transition-all font-medium" :style="draftSettings.editorWidth === 'standard' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">{{ t.widthStandard }}</button>
                <button @click="emit('update:draftSettings', { ...draftSettings, editorWidth: 'full' })" class="flex-1 py-1.5 text-xs rounded-md transition-all font-medium" :style="draftSettings.editorWidth === 'full' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'">{{ t.widthFull }}</button>
              </div>
            </div>
            <div class="flex items-center justify-between px-4 py-3 rounded-xl" style="background: var(--bg-app); border: 1px solid var(--border);">
              <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.fontSize }} ({{ draftSettings.fontSize }}px)</span>
              <input type="range" min="12" max="24" :value="draftSettings.fontSize" @input="emit('update:draftSettings', { ...draftSettings, fontSize: Number(($event.target as HTMLInputElement).value) })" class="w-32" style="accent-color: var(--accent);" />
            </div>
            <div class="flex items-center justify-between px-4 py-3 rounded-xl" style="background: var(--bg-app); border: 1px solid var(--border);">
              <div>
                <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.typewriterMode }}</span>
                <p class="ts-xs mt-0.5" style="color: var(--text-muted);">{{ t.typewriterModeDesc }}</p>
              </div>
              <ToggleSwitch :modelValue="draftSettings.typewriterMode" :label="String(t.typewriterMode)" @update:modelValue="emit('update:draftSettings', { ...draftSettings, typewriterMode: $event })" />
            </div>
          </template>

          <!-- Security tab -->
          <template v-if="settingsTab === 'security'">
            <template v-if="!isKeylessModeActive">
              <div class="px-4 py-3 rounded-xl flex items-center justify-between" style="background: rgba(94,106,210,0.05); border: 1px solid rgba(94,106,210,0.15);">
                <div class="pr-4">
                  <span class="text-sm font-bold" style="color: var(--accent);">{{ t.serverEncrypt }}</span>
                  <p class="ts-xs leading-tight mt-0.5" style="color: var(--text-muted);">{{ t.serverEncryptDesc }}</p>
                </div>
                <ToggleSwitch :modelValue="draftSettings.serverEncrypt" testId="server-encrypt-toggle" :label="String(t.serverEncrypt)" @update:modelValue="emit('update:draftSettings', { ...draftSettings, serverEncrypt: $event })" />
              </div>
              <div v-if="draftSettings.serverEncrypt" class="px-3 py-2 rounded-lg ts-xs leading-snug" style="background: rgba(255,170,0,0.08); border: 1px solid rgba(255,170,0,0.3); color: var(--text-muted);">{{ t.serverEncryptWebDavWarn }}</div>
              <div v-if="draftSettings.serverEncrypt" class="px-3 py-2 rounded-lg ts-xs leading-snug" style="background: var(--color-warning-light); border: 1px solid rgba(217,119,6,0.2); color: var(--color-warning);">{{ t.serverEncryptHistoryWarn }}</div>
              <div class="px-4 py-3 rounded-xl flex items-center justify-between" style="background: var(--bg-app); border: 1px solid var(--border);">
                <div class="pr-4">
                  <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.allowExternalImages }}</span>
                  <p class="ts-xs leading-tight mt-0.5" style="color: var(--text-muted);">{{ t.allowExternalImagesDesc }}</p>
                </div>
                <ToggleSwitch :modelValue="draftSettings.allowExternalImages" :label="String(t.allowExternalImages)" @update:modelValue="emit('update:draftSettings', { ...draftSettings, allowExternalImages: $event })" />
              </div>
              <div class="flex items-center justify-between px-4 py-3 rounded-xl gap-4" style="background: var(--bg-app); border: 1px solid var(--border);">
                <span class="text-sm font-bold shrink-0" style="color: var(--text-primary);">{{ t.libTimeoutLabel }}</span>
                <select data-testid="idle-timeout-select" :value="draftSettings.idleTimeout" @change="emit('update:draftSettings', { ...draftSettings, idleTimeout: Number(($event.target as HTMLSelectElement).value) })" class="p-1.5 rounded-lg border outline-none text-sm" style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit;">
                  <option :value="0">{{ t.libTimeoutNever }}</option>
                  <option :value="0.001">{{ t.libTimeoutImmediate }}</option>
                  <option :value="1">1 Min</option>
                  <option :value="5">5 Min</option>
                  <option :value="30">30 Min</option>
                </select>
              </div>
              <!-- WebDAV access -->
              <div class="px-4 py-3 rounded-xl space-y-2" style="background: var(--bg-app); border: 1px solid var(--border);">
                <div>
                  <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.webdavTitle }}</span>
                  <p class="ts-xs mt-0.5" style="color: var(--text-muted);">{{ t.webdavDesc }}</p>
                </div>
                <div v-if="draftSettings.serverEncrypt" class="px-3 py-2 rounded-lg ts-xs leading-snug" style="background: rgba(255,170,0,0.08); border: 1px solid rgba(255,170,0,0.3); color: var(--text-muted);">{{ t.serverEncryptWebDavWarn }}</div>
                <div class="space-y-1.5">
                  <div class="flex items-center gap-2">
                    <span class="ts-xs shrink-0" style="color: var(--text-muted); width: 4.5rem;">Server URL</span>
                    <code class="flex-1 ts-xs font-mono px-2 py-1.5 rounded-lg truncate" style="background: var(--bg-hover); color: var(--text-primary);">{{ mcpBaseUrl }}/dav/</code>
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="ts-xs shrink-0" style="color: var(--text-muted); width: 4.5rem;">Username</span>
                    <code class="flex-1 ts-xs font-mono px-2 py-1.5 rounded-lg" style="background: var(--bg-hover); color: var(--text-primary);">yinmonote</code>
                  </div>
                </div>
                <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.webdavToken }}</span>
                <!-- Token shown once after generation -->
                <template v-if="webdavTokenValue">
                  <div class="px-3 py-2 rounded-lg ts-xs leading-snug" style="background: rgba(22,163,74,0.08); border: 1px solid rgba(22,163,74,0.3); color: var(--color-success);">{{ t.webdavTokenWarning }}</div>
                  <div class="flex items-center gap-2">
                    <code class="flex-1 ts-xs font-mono px-2 py-1.5 rounded-lg truncate" style="background: var(--bg-hover); color: var(--text-primary);">{{ webdavTokenValue }}</code>
                    <button @click="copyWebDAVToken" class="shrink-0 px-3 py-1.5 rounded-lg text-xs font-medium transition-all" style="background: var(--accent-light); color: var(--accent);">
                      {{ webdavTokenCopied ? t.webdavTokenCopied : 'Copy' }}
                    </button>
                  </div>
                </template>
                <p v-else-if="!webdavTokenSet" class="ts-xs" style="color: var(--text-muted);">{{ t.webdavTokenNone }}</p>
                <p v-else class="ts-xs" style="color: var(--text-muted);">••••••••••••••••</p>
                <div class="flex gap-2 pt-1">
                  <button @click="emit('webdav-generate-token')" :disabled="webdavTokenLoading" class="flex-1 py-1.5 rounded-lg text-xs font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed" style="background: var(--accent-light); color: var(--accent); border: 1px solid rgba(94,106,210,0.2);">{{ t.webdavTokenGenerate }}</button>
                  <button v-if="webdavTokenSet || webdavTokenValue" @click="emit('webdav-revoke-token')" :disabled="webdavTokenLoading" class="flex-1 py-1.5 rounded-lg text-xs font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed" style="background: rgba(239,68,68,0.08); color: var(--color-danger); border: 1px solid rgba(239,68,68,0.2);">{{ t.webdavTokenRevoke }}</button>
                </div>
                <p v-if="webdavTokenError" class="ts-xs pt-1" style="color: var(--color-danger);">{{ webdavTokenError === 'generate_failed' ? t.webdavTokenGenerateFailed : t.webdavTokenRevokeFailed }}</p>
              </div>
              <!-- Change password (password mode only) -->
              <div v-if="!resetIsHardware && !isKeylessModeActive" class="px-4 py-3 rounded-xl space-y-3" style="background: var(--bg-app); border: 1px solid var(--border);">
                <div>
                  <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.changePassword }}</span>
                  <p class="ts-xs mt-0.5" style="color: var(--text-muted);">{{ t.changePasswordDesc }}</p>
                </div>
                <input v-model="cpCurrent" type="password" :placeholder="String(t.currentPassword)"
                  class="w-full px-3 py-2 rounded-lg border text-sm outline-none font-mono"
                  style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary);" />
                <input v-model="cpNew" type="password" :placeholder="String(t.newPassword)"
                  class="w-full px-3 py-2 rounded-lg border text-sm outline-none font-mono"
                  style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary);" />
                <input v-model="cpConfirm" type="password" :placeholder="String(t.confirmNewPassword)"
                  class="w-full px-3 py-2 rounded-lg border text-sm outline-none font-mono"
                  style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary);"
                  @keydown.enter="emitChangePassword" />
                <p v-if="cpError" class="ts-xs font-medium" style="color: var(--color-danger);">{{ cpError }}</p>
                <p v-if="cpSuccess" class="ts-xs font-medium" style="color: var(--color-success);">{{ cpSuccess }}</p>
                <button @click="emitChangePassword" :disabled="cpLoading || !cpCurrent || !cpNew || !cpConfirm"
                  class="w-full py-2 rounded-lg text-sm font-semibold transition-all disabled:opacity-50"
                  style="background: var(--accent); color: white;">
                  {{ cpLoading ? '...' : t.changePassword }}
                </button>
              </div>
              <!-- Export key (hardware mode only — password mode can re-derive the key from the password) -->
              <div v-if="resetIsHardware" class="px-4 py-3 rounded-xl space-y-2" style="background: var(--bg-app); border: 1px solid var(--border);">
                <div class="flex items-center justify-between gap-4">
                  <div class="min-w-0">
                    <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.libExportKeyTitle }}</span>
                    <p class="ts-xs leading-tight mt-0.5" style="color: var(--text-muted);">{{ t.libExportKeyDesc }}</p>
                  </div>
                  <button @click="emit('export-key')" class="shrink-0 px-4 py-1.5 rounded-lg text-xs font-bold transition-all" style="background: var(--accent-light); color: var(--accent); border: 1px solid rgba(94,106,210,0.2);">
                    {{ t.libExportAction }}
                  </button>
                </div>
                <p v-if="exportKeyStatus === 'success'" class="ts-xs font-medium" style="color: var(--color-success);">{{ t.libExportSuccess }}</p>
                <p v-else-if="exportKeyStatus === 'error'" class="ts-xs font-medium" style="color: var(--color-danger);">{{ t.libExportFailed }}</p>
                <div v-else-if="exportKeyStatus === 'fallback'" class="space-y-1">
                  <p class="ts-xs font-medium" style="color: var(--color-warning);">{{ t.libExportFallback ?? 'Clipboard unavailable — copy the key below:' }}</p>
                  <textarea readonly :value="exportedKeyText" class="w-full ts-xs font-mono rounded-lg px-2 py-1.5 resize-none" rows="3" style="background: var(--bg-hover); color: var(--text-primary); border: 1px solid var(--border);" @click="(e) => (e.target as HTMLTextAreaElement).select()"></textarea>
                </div>
              </div>
            </template>
            <!-- Reset library -->
            <div class="pt-1 space-y-2">
              <button data-testid="reset-library-btn" @click="emit('open-reset-modal')" class="w-full py-2.5 rounded-xl text-sm font-bold transition-all active:scale-[0.98]" style="background: var(--color-danger); color: white;">
                {{ t.libResetBtn }}
              </button>
              <p v-if="batchResultMsg" class="ts-xs font-medium text-center px-2" style="color: var(--color-warning);">{{ batchResultMsg }}</p>
            </div>
          </template>

          <!-- AI Access tab -->
          <template v-if="settingsTab === 'ai'">
            <!-- Enable MCP toggle -->
            <div class="px-4 py-3 rounded-xl flex items-center justify-between" style="background: rgba(94,106,210,0.05); border: 1px solid rgba(94,106,210,0.15);">
              <div class="pr-4">
                <span class="text-sm font-bold" style="color: var(--accent);">{{ t.mcpEnabled }}</span>
                <p class="ts-xs leading-tight mt-0.5" style="color: var(--text-muted);">{{ t.mcpEnabledDesc }}</p>
              </div>
              <ToggleSwitch :modelValue="draftSettings.mcpEnabled" testId="mcp-enabled-toggle" :label="String(t.mcpEnabledLabel ?? 'MCP')" @update:modelValue="emit('update:draftSettings', { ...draftSettings, mcpEnabled: $event })" />
            </div>

            <template v-if="draftSettings.mcpEnabled">
              <!-- Endpoint display -->
              <div class="px-4 py-3 rounded-xl space-y-1" style="background: var(--bg-app); border: 1px solid var(--border);">
                <span class="text-xs font-bold" style="color: var(--text-muted);">{{ t.mcpEndpoint }}</span>
                <div class="flex items-center gap-2">
                  <code class="flex-1 ts-xs font-mono px-2 py-1.5 rounded-lg truncate" style="background: var(--bg-hover); color: var(--text-primary);">{{ mcpEndpointUrl }}</code>
                </div>
              </div>

              <!-- Token management -->
              <div class="px-4 py-3 rounded-xl space-y-2" style="background: var(--bg-app); border: 1px solid var(--border);">
                <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.mcpToken }}</span>
                <!-- New token display (shown once after generation) -->
                <template v-if="mcpTokenValue">
                  <div class="px-3 py-2 rounded-lg ts-xs leading-snug" style="background: rgba(22,163,74,0.08); border: 1px solid rgba(22,163,74,0.3); color: var(--color-success);">{{ t.mcpTokenWarning }}</div>
                  <div class="flex items-center gap-2">
                    <code class="flex-1 ts-xs font-mono px-2 py-1.5 rounded-lg truncate" style="background: var(--bg-hover); color: var(--text-primary);">{{ mcpTokenValue }}</code>
                    <button @click="copyToken" class="shrink-0 px-3 py-1.5 rounded-lg text-xs font-medium transition-all" style="background: var(--accent-light); color: var(--accent);">
                      {{ tokenCopied ? t.mcpTokenCopied : 'Copy' }}
                    </button>
                  </div>
                </template>
                <p v-else-if="!mcpTokenSet" class="ts-xs" style="color: var(--text-muted);">{{ t.mcpTokenNone }}</p>
                <p v-else class="ts-xs" style="color: var(--text-muted);">••••••••••••••••</p>
                <div class="flex gap-2 pt-1">
                  <button data-testid="mcp-generate-token-btn" @click="emit('mcp-generate-token')" :disabled="mcpTokenLoading" class="flex-1 py-1.5 rounded-lg text-xs font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed" style="background: var(--accent-light); color: var(--accent); border: 1px solid rgba(94,106,210,0.2);">{{ t.mcpTokenGenerate }}</button>
                  <button v-if="mcpTokenSet || mcpTokenValue" data-testid="mcp-revoke-token-btn" @click="emit('mcp-revoke-token')" :disabled="mcpTokenLoading" class="flex-1 py-1.5 rounded-lg text-xs font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed" style="background: rgba(239,68,68,0.08); color: var(--color-danger); border: 1px solid rgba(239,68,68,0.2);">{{ t.mcpTokenRevoke }}</button>
                </div>
                <p v-if="mcpTokenError" class="ts-xs pt-1" style="color: var(--color-danger);">{{ mcpTokenError === 'generate_failed' ? t.mcpTokenGenerateFailed : t.mcpTokenRevokeFailed }}</p>
              </div>

              <!-- CA fingerprint (only shown when TLS_SELF is active) -->
              <template v-if="mcpCaFingerprint">
                <div class="px-4 py-3 rounded-xl space-y-2" style="background: var(--bg-app); border: 1px solid var(--border);">
                  <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.mcpCaInstall }}</span>
                  <p class="ts-xs leading-snug" style="color: var(--text-muted);">{{ t.mcpCaFingerprintDesc }}</p>
                  <div class="px-2 py-1.5 rounded-lg" style="background: var(--bg-hover);">
                    <p class="ts-xs font-bold mb-0.5" style="color: var(--text-muted);">{{ t.mcpCaFingerprint }}</p>
                    <code class="ts-xs font-mono break-all" style="color: var(--text-primary);">{{ mcpCaFingerprint }}</code>
                    <p v-if="mcpCaExpiry" class="ts-xs mt-1" style="color: var(--text-muted);">Expires: {{ mcpCaExpiry }}</p>
                  </div>
                  <!-- Install instructions by OS -->
                  <details class="ts-xs" style="color: var(--text-muted);">
                    <summary class="cursor-pointer font-medium select-none py-1">{{ t.mcpCaInstallMac }}</summary>
                    <pre class="mt-1 px-2 py-1.5 rounded-lg overflow-x-auto ts-xs font-mono" style="background: var(--bg-hover); color: var(--text-primary);">curl -k -o ca.crt {{ mcpBaseUrl }}/ca.crt
# Verify fingerprint matches above, then:
# macOS:
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain ca.crt
# Linux (Debian/Ubuntu):
sudo cp ca.crt /usr/local/share/ca-certificates/yinmo-ca.crt
sudo update-ca-certificates</pre>
                  </details>
                  <details class="ts-xs" style="color: var(--text-muted);">
                    <summary class="cursor-pointer font-medium select-none py-1">{{ t.mcpCaInstallWin }}</summary>
                    <pre class="mt-1 px-2 py-1.5 rounded-lg overflow-x-auto ts-xs font-mono" style="background: var(--bg-hover); color: var(--text-primary);">curl -k -o ca.crt {{ mcpBaseUrl }}/ca.crt
# Verify fingerprint, then double-click ca.crt
# → Install Certificate → Local Machine
# → Place in "Trusted Root Certification Authorities"</pre>
                  </details>
                  <details class="ts-xs" style="color: var(--text-muted);">
                    <summary class="cursor-pointer font-medium select-none py-1">{{ t.mcpCaInstallIos }}</summary>
                    <pre class="mt-1 px-2 py-1.5 rounded-lg overflow-x-auto ts-xs font-mono" style="background: var(--bg-hover); color: var(--text-primary);">1. Open {{ mcpBaseUrl }}/ca.crt in Safari
2. Install profile → Settings → Downloaded Profile
3. Settings → General → VPN & Device Management
   → Trust the certificate</pre>
                  </details>
                  <details class="ts-xs" style="color: var(--text-muted);">
                    <summary class="cursor-pointer font-medium select-none py-1">{{ t.mcpCaInstallAndroid }}</summary>
                    <pre class="mt-1 px-2 py-1.5 rounded-lg overflow-x-auto ts-xs font-mono" style="background: var(--bg-hover); color: var(--text-primary);">1. Open {{ mcpBaseUrl }}/ca.crt in Chrome
2. Settings → Security → Encryption & credentials
   → Install a certificate → CA certificate</pre>
                  </details>
                </div>
              </template>

              <!-- Default access policy -->
              <div class="flex items-center justify-between px-4 py-3 rounded-xl gap-4" style="background: var(--bg-app); border: 1px solid var(--border);">
                <span class="text-sm font-bold shrink-0" style="color: var(--text-primary);">{{ t.mcpDefaultAccess }}</span>
                <select :value="draftSettings.mcpDefaultAccess"
                  @change="emit('update:draftSettings', { ...draftSettings, mcpDefaultAccess: ($event.target as HTMLSelectElement).value })"
                  class="p-1.5 rounded-lg border outline-none text-sm"
                  style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit;">
                  <option value="read">{{ t.mcpAccessRead }}</option>
                  <option value="write">{{ t.mcpAccessWrite }}</option>
                  <option value="deny">{{ t.mcpAccessDeny }}</option>
                </select>
              </div>

              <!-- Access rules -->
              <div class="px-4 py-3 rounded-xl space-y-3" style="background: var(--bg-app); border: 1px solid var(--border);">
                <div class="flex items-center justify-between">
                  <div>
                    <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.mcpRules }}</span>
                    <p class="ts-xs mt-0.5" style="color: var(--text-muted);">{{ t.mcpRulesDesc }}</p>
                  </div>
                  <button @click="addRule" class="px-3 py-1 rounded-lg text-xs font-medium transition-all shrink-0" style="background: var(--accent-light); color: var(--accent); border: 1px solid rgba(94,106,210,0.2);">+ {{ t.mcpAddRule }}</button>
                </div>
                <!-- Rule list -->
                <div v-for="(rule, idx) in draftSettings.mcpRules" :key="idx" class="flex items-center gap-2 p-2 rounded-lg" style="background: var(--bg-hover); border: 1px solid var(--border);">
                  <!-- Condition type -->
                  <select :value="rule.condition"
                    @change="updateRule(idx, 'condition', ($event.target as HTMLSelectElement).value)"
                    class="ts-xs px-1.5 py-1 rounded-md border outline-none"
                    style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit; width: 100px; flex-shrink: 0;">
                    <option value="tag">{{ t.mcpRuleTag }}</option>
                    <option value="note_id">{{ t.mcpRuleNoteID }}</option>
                    <option value="title_glob">{{ t.mcpRuleTitleGlob }}</option>
                    <option value="subtree_of">{{ t.mcpRuleSubtreeOf }}</option>
                  </select>
                  <!-- Value -->
                  <input :value="rule.value"
                    @input="updateRule(idx, 'value', ($event.target as HTMLInputElement).value)"
                    :placeholder="t.mcpRuleValue as string"
                    class="flex-1 min-w-0 ts-xs px-2 py-1 rounded-md border outline-none"
                    style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit;" />
                  <!-- Access level -->
                  <select :value="rule.access"
                    @change="updateRule(idx, 'access', ($event.target as HTMLSelectElement).value)"
                    class="ts-xs px-1.5 py-1 rounded-md border outline-none"
                    style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit; width: 80px; flex-shrink: 0;">
                    <option value="read">{{ t.mcpAccessRead }}</option>
                    <option value="write">{{ t.mcpAccessWrite }}</option>
                    <option value="deny">{{ t.mcpAccessDeny }}</option>
                  </select>
                  <!-- Remove -->
                  <button @click="removeRule(idx)" class="shrink-0 w-6 h-6 flex items-center justify-center rounded-md transition-all" style="color: var(--color-danger); background: rgba(239,68,68,0.08);">
                    <svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
                  </button>
                </div>
                <p v-if="draftSettings.mcpRules.length === 0" class="ts-xs text-center py-2" style="color: var(--text-muted);">—</p>
              </div>
            </template>
          </template>
        </div>
        <!-- Footer buttons -->
        <div class="px-6 py-4 shrink-0" style="border-top: 1px solid var(--border);">
          <!-- Unsaved changes confirm banner -->
          <div v-if="showSettingsCloseConfirm" class="mb-3 px-4 py-3 rounded-xl flex items-center justify-between gap-3" style="background: rgba(245,158,11,0.08); border: 1px solid rgba(245,158,11,0.25);">
            <span class="ts-sm font-medium" style="color: var(--color-warning);">{{ t.unsavedChanges }}</span>
            <div class="flex gap-2 shrink-0">
              <button @click="emit('update:showSettingsCloseConfirm', false)" class="px-3 py-1 rounded-lg text-xs font-medium transition-all" style="background: var(--bg-hover); color: var(--text-secondary);">{{ t.close }}</button>
              <button @click="emit('force-close')" class="px-3 py-1 rounded-lg text-xs font-semibold transition-all" style="background: var(--color-warning); color: white;">{{ t.settingsDiscardClose }}</button>
            </div>
          </div>
          <p v-if="settingsSaveError" class="ts-xs font-medium text-center mb-2" style="color: var(--color-danger);">{{ t.settingsSaveFailed ?? 'Failed to save settings to server' }}</p>
          <div class="flex justify-between">
            <button @click="emit('close')" class="px-4 py-2 rounded-lg text-sm font-medium transition-all focus-ring" style="color: var(--text-secondary); background: var(--bg-hover);">{{ t.close }}</button>
            <button data-testid="settings-apply-btn" @click="emit('apply')" :disabled="batchProcessing" class="px-5 py-2 rounded-lg text-sm font-semibold transition-all active:scale-95 disabled:opacity-50 focus-ring" style="background: var(--accent); color: white;">{{ t.apply }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import ToggleSwitch from './ToggleSwitch.vue'

export interface MCPDraftRule {
  condition: string  // 'tag' | 'note_id' | 'title_glob' | 'subtree_of'
  value: string
  access: string     // 'read' | 'write' | 'deny'
}

interface DraftSettings {
  themeMode: string
  lang: string
  fontSize: number
  editorWidth: string
  typewriterMode: boolean
  serverEncrypt: boolean
  idleTimeout: number
  allowExternalImages: boolean
  mcpEnabled: boolean
  mcpDefaultAccess: string
  mcpRules: MCPDraftRule[]
}

const props = defineProps<{
  modelValue: boolean
  t: Record<string, unknown>
  draftSettings: DraftSettings
  batchProcessing: boolean
  showSettingsCloseConfirm: boolean
  exportKeyStatus: string
  exportedKeyText?: string
  batchResultMsg: string
  settingsSaveError?: boolean
  resetIsHardware: boolean
  isKeylessModeActive: boolean
  settingsTab: 'appearance' | 'editor' | 'security' | 'ai'
  mcpTokenValue?: string
  mcpTokenSet: boolean
  mcpTokenLoading?: boolean
  mcpTokenError?: string
  mcpCaFingerprint?: string
  mcpCaExpiry?: string
  webdavTokenValue?: string
  webdavTokenSet: boolean
  webdavTokenLoading?: boolean
  webdavTokenError?: string
}>()

const emit = defineEmits<{
  'close': []
  'apply': []
  'force-close': []
  'open-reset-modal': []
  'export-key': []
  'mcp-generate-token': []
  'mcp-revoke-token': []
  'webdav-generate-token': []
  'webdav-revoke-token': []
  'update:draftSettings': [value: DraftSettings]
  'update:showSettingsCloseConfirm': [value: boolean]
  'update:settingsTab': [value: 'appearance' | 'editor' | 'security' | 'ai']
  'change-password': [currentPassword: string, newPassword: string]
}>()

// Change password state
const cpCurrent = ref('')
const cpNew = ref('')
const cpConfirm = ref('')
const cpError = ref('')
const cpSuccess = ref('')
const cpLoading = ref(false)

const emitChangePassword = () => {
  cpError.value = ''; cpSuccess.value = ''
  if (cpNew.value !== cpConfirm.value) {
    cpError.value = String(props.t.passwordMismatch)
    return
  }
  if (cpNew.value.length < 1) return
  cpLoading.value = true
  emit('change-password', cpCurrent.value, cpNew.value)
}

// Called by parent after password change completes or fails
const onPasswordChangeResult = (success: boolean, msg?: string) => {
  cpLoading.value = false
  if (success) {
    cpSuccess.value = msg || String(props.t.passwordChanged)
    cpCurrent.value = ''; cpNew.value = ''; cpConfirm.value = ''
  } else {
    cpError.value = msg || String(props.t.currentPasswordWrong)
  }
}

defineExpose({ onPasswordChangeResult })

const tokenCopied = ref(false)
let tokenCopiedTimer: ReturnType<typeof setTimeout> | null = null

const webdavTokenCopied = ref(false)
let webdavTokenCopiedTimer: ReturnType<typeof setTimeout> | null = null

const mcpBaseUrl = computed(() => window.location.origin)
const mcpEndpointUrl = computed(() => `${window.location.origin}/mcp/sse`)

onUnmounted(() => {
  if (tokenCopiedTimer !== null) { clearTimeout(tokenCopiedTimer); tokenCopiedTimer = null }
  if (webdavTokenCopiedTimer !== null) { clearTimeout(webdavTokenCopiedTimer); webdavTokenCopiedTimer = null }
})

function copyWebDAVToken() {
  if (!props.webdavTokenValue) return
  navigator.clipboard.writeText(props.webdavTokenValue).then(() => {
    webdavTokenCopied.value = true
    if (webdavTokenCopiedTimer !== null) clearTimeout(webdavTokenCopiedTimer)
    webdavTokenCopiedTimer = setTimeout(() => { webdavTokenCopied.value = false; webdavTokenCopiedTimer = null }, 2000)
  }).catch(() => {
    console.error('[YinMo] Failed to copy WebDAV token to clipboard')
  })
}

function copyToken() {
  if (!props.mcpTokenValue) return
  navigator.clipboard.writeText(props.mcpTokenValue).then(() => {
    tokenCopied.value = true
    if (tokenCopiedTimer !== null) clearTimeout(tokenCopiedTimer)
    tokenCopiedTimer = setTimeout(() => { tokenCopied.value = false; tokenCopiedTimer = null }, 2000)
  }).catch(() => {
    console.error('[YinMo] Failed to copy token to clipboard')
  })
}

function addRule() {
  emit('update:draftSettings', {
    ...props.draftSettings,
    mcpRules: [...props.draftSettings.mcpRules, { condition: 'tag', value: '', access: 'read' }],
  })
}

function removeRule(idx: number) {
  const rules = props.draftSettings.mcpRules.filter((_, i) => i !== idx)
  emit('update:draftSettings', { ...props.draftSettings, mcpRules: rules })
}

function updateRule(idx: number, field: keyof MCPDraftRule, val: string) {
  const rules = props.draftSettings.mcpRules.map((r, i) => i === idx ? { ...r, [field]: val } : r)
  emit('update:draftSettings', { ...props.draftSettings, mcpRules: rules })
}
</script>
