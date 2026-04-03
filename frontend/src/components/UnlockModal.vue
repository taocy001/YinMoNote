<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="modelValue" class="fixed inset-0 z-[300] flex items-center justify-center p-4 transition-colors duration-500" :style="isDark ? 'background: #0D0D10;' : 'background: #F7F6F3;'">
        <div class="w-full max-w-md rounded-3xl p-8 space-y-8" style="background: var(--bg-editor); box-shadow: var(--shadow-lg);">
          <!-- Logo + Title -->
          <div class="text-center space-y-2">
            <div class="w-16 h-16 rounded-2xl flex items-center justify-center mx-auto" style="background: var(--accent-light); color: var(--accent);">
              <svg width="32" height="32" viewBox="0 0 18 18" fill="none"><path d="M9 2C9 2 3.5 7.5 3.5 11a5.5 5.5 0 0 0 11 0C14.5 7.5 9 2 9 2z" fill="currentColor" opacity="0.9"/><ellipse cx="11" cy="10" rx="1.5" ry="2" fill="white" opacity="0.35"/></svg>
            </div>
            <h2 class="text-3xl font-black tracking-tight" style="color: var(--text-primary);">
              <template v-if="!hasLibraryKey">{{ t.libInitTitle }}</template>
              <template v-else-if="unlockMode === 'device'">{{ t.libUnlockDeviceTitle }}</template>
              <template v-else>{{ t.libUnlockPasswordTitle }}</template>
            </h2>
          </div>

          <!-- ── UNLOCK (library already initialized) ── -->
          <template v-if="hasLibraryKey">
            <!-- Device / biometric unlock -->
            <div v-if="unlockMode === 'device'" class="space-y-6">
              <div class="flex flex-col items-center gap-4 py-2">
                <div class="w-24 h-24 rounded-3xl flex items-center justify-center" style="background: var(--accent-light); color: var(--accent);">
                  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M12 10a2 2 0 0 0-2 2c0 1.02-.1 2.51-.26 4"/><path d="M14 13.12c0 2.38 0 6.38-1 8.88"/>
                    <path d="M17.29 21.02c.12-.6.43-2.3.5-3.02"/><path d="M2 12a10 10 0 0 1 18-6"/>
                    <path d="M2 17.5a14.5 14.5 0 0 0 4.24 5.18"/><path d="M6 10.5a10 10 0 0 1 6-3.5"/>
                    <path d="M22 12c0 .34-.02.67-.05 1"/><path d="M7 22.67A10 10 0 0 1 4.35 18"/>
                    <path d="M9.08 19.84A10 10 0 0 1 8.18 14"/>
                  </svg>
                </div>
                <p class="text-sm text-center leading-relaxed" style="color: var(--text-muted);">{{ t.libUnlockDeviceHint }}</p>
              </div>
              <button data-testid="unlock-btn" :disabled="isUnlocking" class="group relative w-full py-4 rounded-2xl font-black tracking-widest transition-all active:scale-[0.98] disabled:opacity-50" style="background: var(--accent); color: white; box-shadow: 0 8px 24px rgba(94,106,210,0.35);" @click="emit('unlock')">
                <span v-if="!isUnlocking" class="inline-flex items-center gap-2">{{ t.libVerifyIdentityBtn }}<span class="group-hover:translate-x-1 transition-transform">→</span></span>
                <span v-else class="flex items-center justify-center"><span class="w-5 h-5 border-2 rounded-full animate-spin" style="border-color: rgba(255,255,255,0.3); border-top-color: white;"></span></span>
              </button>
            </div>
            <!-- Password unlock -->
            <div v-else class="space-y-4">
              <div class="relative">
                <input ref="unlockPasswordRef" data-testid="unlock-password-input" :value="unlockPassword" :type="showPassword ? 'text' : 'password'" placeholder="" class="w-full px-5 py-4 pr-12 rounded-2xl outline-none transition-all font-mono" style="border: 2px solid var(--border); background: transparent; color: var(--text-primary);" @input="emit('update:unlockPassword', ($event.target as HTMLInputElement).value)" @keydown.enter="emit('unlock')" />
                <button type="button" class="absolute right-4 top-1/2 -translate-y-1/2 transition-opacity" :style="{ opacity: showPassword ? '1' : '0.4', color: 'var(--text-muted)' }" tabindex="-1" @click="emit('update:showPassword', !showPassword)">
                  <svg v-if="!showPassword" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                </button>
              </div>
              <button data-testid="unlock-btn" :disabled="isUnlocking" class="group relative w-full py-4 rounded-2xl font-black tracking-widest transition-all active:scale-[0.98] disabled:opacity-50" style="background: var(--accent); color: white; box-shadow: 0 8px 24px rgba(94,106,210,0.35);" @click="emit('unlock')">
                <span v-if="!isUnlocking" class="inline-flex items-center gap-2">{{ t.libUnlockAction }}<span class="group-hover:translate-x-1 transition-transform">→</span></span>
                <span v-else class="flex items-center justify-center"><span class="w-5 h-5 border-2 rounded-full animate-spin" style="border-color: rgba(255,255,255,0.3); border-top-color: white;"></span></span>
              </button>
            </div>
          </template>

          <!-- ── INIT (first-time setup) ── -->
          <template v-else>
            <div class="space-y-6">
              <!-- Explanation -->
              <div class="space-y-3 pt-4" style="border-top: 1px solid var(--border);">
                <div class="space-y-1">
                  <h4 class="ts-xs font-black uppercase tracking-widest" style="color: var(--accent);">{{ t.libWhyInit }}</h4>
                  <p class="ts-xs leading-relaxed" style="color: var(--text-muted);">{{ t.libInitReason }}</p>
                </div>
                <div class="space-y-1.5">
                  <h4 class="ts-xs font-black uppercase tracking-widest" style="color: var(--accent);">{{ t.libMethodDiff }}</h4>
                  <p class="ts-xs leading-relaxed" style="color: var(--text-muted);">{{ t.libMethodBiometricsDesc }}</p>
                  <p class="ts-xs leading-relaxed" style="color: var(--text-muted);">{{ t.libMethodPasswordDesc }}</p>
                  <p class="ts-xs leading-relaxed" style="color: var(--text-muted);">{{ t.libMethodNoneDesc }}</p>
                </div>
              </div>
              <!-- Tab row -->
              <div class="flex p-1.5 rounded-2xl" style="background: var(--bg-app);">
                <button data-testid="mode-tab-none" class="flex-1 py-2.5 text-sm rounded-xl transition-all duration-300 font-medium" :style="unlockMode === 'none' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'" @click="emit('update:unlockMode', 'none')">{{ t.libModeNone }}</button>
                <button data-testid="mode-tab-password" class="flex-1 py-2.5 text-sm rounded-xl transition-all duration-300 font-medium" :style="unlockMode === 'password' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'" @click="emit('update:unlockMode', 'password')">{{ t.libModePassword }}</button>
                <button v-if="hasServerNotes" data-testid="mode-tab-import" class="flex-1 py-2.5 text-sm rounded-xl transition-all duration-300 font-medium" :style="unlockMode === 'import' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'" @click="emit('update:unlockMode', 'import')">{{ t.libModeImport }}</button>
                <button v-if="webauthnAvailable" data-testid="mode-tab-device" class="flex-1 py-2.5 text-sm rounded-xl transition-all duration-300 font-medium" :style="unlockMode === 'device' ? 'background: var(--bg-editor); color: var(--text-primary); box-shadow: var(--shadow-sm);' : 'color: var(--text-muted);'" @click="emit('update:unlockMode', 'device')">{{ t.libModeDevice }}</button>
              </div>
              <!-- Mode content -->
              <div class="space-y-3">
                <!-- Device: fingerprint icon + hint + recommended badge -->
                <div v-if="unlockMode === 'device'" class="flex flex-col items-center gap-3">
                  <div class="w-16 h-16 rounded-2xl flex items-center justify-center" style="background: var(--accent-light); color: var(--accent);">
                    <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M12 10a2 2 0 0 0-2 2c0 1.02-.1 2.51-.26 4"/><path d="M14 13.12c0 2.38 0 6.38-1 8.88"/>
                      <path d="M17.29 21.02c.12-.6.43-2.3.5-3.02"/><path d="M2 12a10 10 0 0 1 18-6"/>
                      <path d="M2 17.5a14.5 14.5 0 0 0 4.24 5.18"/><path d="M6 10.5a10 10 0 0 1 6-3.5"/>
                      <path d="M22 12c0 .34-.02.67-.05 1"/><path d="M7 22.67A10 10 0 0 1 4.35 18"/>
                      <path d="M9.08 19.84A10 10 0 0 1 8.18 14"/>
                    </svg>
                  </div>
                  <p class="ts-xs text-center leading-relaxed" style="color: var(--text-muted);">{{ t.libInitDeviceHint }}</p>
                </div>
                <!-- Password: hint + input + warning -->
                <div v-else-if="unlockMode === 'password'" class="space-y-2 animate-in fade-in slide-in-from-top-2">
                  <p class="ts-xs text-center leading-relaxed" style="color: var(--text-muted);">{{ t.libInitPasswordHint }}</p>
                  <div class="relative">
                    <input data-testid="unlock-password-input" :value="unlockPassword" :type="showPassword ? 'text' : 'password'" placeholder="" class="w-full px-5 py-4 pr-12 rounded-2xl outline-none transition-all font-mono" style="border: 2px solid var(--border); background: transparent; color: var(--text-primary);" @input="emit('update:unlockPassword', ($event.target as HTMLInputElement).value)" @keydown.enter="emit('unlock')" />
                    <button type="button" class="absolute right-4 top-1/2 -translate-y-1/2 transition-opacity" :style="{ opacity: showPassword ? '1' : '0.4', color: 'var(--text-muted)' }" tabindex="-1" @click="emit('update:showPassword', !showPassword)">
                      <svg v-if="!showPassword" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                      <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                    </button>
                  </div>
                  <p class="ts-xs text-center font-medium" style="color: var(--color-warning);">{{ t.libPasswordWarning }}</p>
                </div>
                <!-- Import: hint + textarea -->
                <div v-else-if="unlockMode === 'import'" class="space-y-2 animate-in fade-in slide-in-from-top-2">
                  <p class="ts-xs text-center leading-relaxed" style="color: var(--text-muted);">{{ t.libImportKeyHint }}</p>
                  <textarea :value="importKeyText" :placeholder="(t.libImportKeyPlaceholder as string)" class="w-full px-5 py-4 rounded-2xl outline-none transition-all h-28 text-xs font-mono resize-none" style="border: 2px solid var(--border); background: transparent; color: var(--text-primary);" @input="emit('update:importKeyText', ($event.target as HTMLTextAreaElement).value)"></textarea>
                </div>
                <!-- None: hint -->
                <div v-else-if="unlockMode === 'none'">
                  <p class="ts-xs text-center leading-relaxed" style="color: var(--text-muted);">{{ t.libNoneHint }}</p>
                </div>
              </div>
              <!-- Action button with mode-specific label -->
              <button data-testid="unlock-btn" :disabled="isUnlocking" class="group relative w-full py-4 rounded-2xl font-black tracking-widest transition-all active:scale-[0.98] disabled:opacity-50" style="background: var(--accent); color: white; box-shadow: 0 8px 24px rgba(94,106,210,0.35);" @click="emit('unlock')">
                <span v-if="!isUnlocking" class="inline-flex items-center gap-2">
                  <template v-if="unlockMode === 'device'">{{ t.libInitDeviceBtn }}</template>
                  <template v-else-if="unlockMode === 'import'">{{ t.libInitImportBtn }}</template>
                  <template v-else-if="unlockMode === 'none'">{{ t.libInitNoneBtn }}</template>
                  <template v-else>{{ t.libInitPasswordBtn }}</template>
                  <span class="group-hover:translate-x-1 transition-transform">→</span>
                </span>
                <span v-else class="flex items-center justify-center"><span class="w-5 h-5 border-2 rounded-full animate-spin" style="border-color: rgba(255,255,255,0.3); border-top-color: white;"></span></span>
              </button>
            </div>
          </template>

          <p v-if="unlockError" data-testid="unlock-error" class="text-sm text-center font-black animate-shake" style="color: var(--color-danger);">{{ unlockErrorMsg || t.libError }}</p>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  modelValue: boolean
  isDark: boolean
  hasLibraryKey: boolean
  hasServerNotes: boolean
  unlockMode: 'password' | 'device' | 'import' | 'none'
  unlockPassword: string
  importKeyText: string
  isUnlocking: boolean
  unlockError: boolean
  unlockErrorMsg: string
  showPassword: boolean
  t: Record<string, unknown>
}>()

const emit = defineEmits<{
  'unlock': []
  'update:unlockMode': [value: 'password' | 'device' | 'import' | 'none']
  'update:unlockPassword': [value: string]
  'update:importKeyText': [value: string]
  'update:showPassword': [value: boolean]
}>()

// WebAuthn requires a domain name — hide the device/biometric tab when
// accessed via IP address or localhost (browsers reject RP ID for IPs).
const webauthnAvailable = computed(() => {
  const h = window.location.hostname
  return !!window.PublicKeyCredential && !/^\d+\.\d+\.\d+\.\d+$/.test(h) && h !== 'localhost'
})

const unlockPasswordRef = ref<HTMLInputElement | null>(null)

watch(() => props.modelValue, (visible) => {
  if (visible && props.hasLibraryKey && props.unlockMode !== 'device') {
    nextTick(() => unlockPasswordRef.value?.focus())
  }
}, { immediate: true })
</script>
