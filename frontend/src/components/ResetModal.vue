<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="modelValue" class="fixed inset-0 z-[400] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(4px);">
        <div class="w-full max-w-sm rounded-2xl overflow-hidden" style="background: var(--bg-editor); box-shadow: var(--shadow-lg); border: 1px solid rgba(239,68,68,0.3);">
          <!-- Header -->
          <div class="px-6 pt-6 pb-4 flex items-center gap-3" style="border-bottom: 1px solid var(--border);">
            <div class="w-9 h-9 rounded-xl flex items-center justify-center shrink-0" style="background: rgba(239,68,68,0.1);">
              <svg width="18" height="18" viewBox="0 0 20 20" fill="none"><path d="M10 2L2 17h16L10 2z" stroke="#EF4444" stroke-width="1.5" stroke-linejoin="round"/><path d="M10 8v4M10 14.5v.5" stroke="#EF4444" stroke-width="1.5" stroke-linecap="round"/></svg>
            </div>
            <span class="font-bold text-base" style="color: var(--color-danger);">{{ t.libResetModalTitle }}</span>
          </div>

          <div class="px-6 py-4 space-y-4">
            <!-- What will happen -->
            <div>
              <p class="ts-xs font-black uppercase tracking-widest mb-2" style="color: var(--text-muted);">{{ t.libResetModalWhat }}</p>
              <ul class="space-y-1.5">
                <li v-for="action in t.libResetModalActions" :key="action" class="flex items-start gap-2">
                  <span class="mt-[3px] shrink-0 w-3.5 h-3.5 rounded-full flex items-center justify-center" style="background: rgba(239,68,68,0.12);">
                    <svg width="7" height="7" viewBox="0 0 8 8"><path d="M2 4h4M4 2v4" stroke="#EF4444" stroke-width="1.5" stroke-linecap="round"/></svg>
                  </span>
                  <span class="ts-sm leading-snug" style="color: var(--text-secondary);">{{ action }}</span>
                </li>
              </ul>
            </div>

            <!-- Impact -->
            <div class="px-4 py-3 rounded-xl space-y-1.5" style="background: rgba(239,68,68,0.05); border: 1px solid rgba(239,68,68,0.15);">
              <p class="ts-xs font-black uppercase tracking-widest mb-2" style="color: rgba(239,68,68,0.7);">{{ t.libResetModalImpact }}</p>
              <p v-for="effect in t.libResetModalEffects" :key="effect" class="ts-xs leading-snug flex items-start gap-1.5">
                <span style="color: var(--color-danger);">⚠</span>
                <span style="color: rgba(239,68,68,0.85);">{{ effect }}</span>
              </p>
            </div>

            <!-- Hardware (biometric) mode: fingerprint icon + explanation -->
            <div v-if="!isKeylessModeActive && resetIsHardware" class="flex flex-col items-center gap-3 py-1">
              <div class="w-14 h-14 rounded-2xl flex items-center justify-center" style="background: var(--accent-light); color: var(--accent);">
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 10a2 2 0 0 0-2 2c0 1.02-.1 2.51-.26 4"/><path d="M14 13.12c0 2.38 0 6.38-1 8.88"/>
                  <path d="M17.29 21.02c.12-.6.43-2.3.5-3.02"/><path d="M2 12a10 10 0 0 1 18-6"/>
                  <path d="M2 17.5a14.5 14.5 0 0 0 4.24 5.18"/><path d="M6 10.5a10 10 0 0 1 6-3.5"/>
                  <path d="M22 12c0 .34-.02.67-.05 1"/><path d="M7 22.67A10 10 0 0 1 4.35 18"/>
                  <path d="M9.08 19.84A10 10 0 0 1 8.18 14"/>
                </svg>
              </div>
              <p class="ts-sm text-center leading-relaxed" style="color: var(--text-muted);">{{ t.libResetHWHint }}</p>
            </div>

            <!-- Password input (non-hardware, non-keyless mode only) -->
            <div v-if="!isKeylessModeActive && !resetIsHardware" class="space-y-1.5">
              <p class="ts-xs font-semibold" style="color: var(--text-muted);">{{ t.libEnterPass }}</p>
              <div class="relative">
                <input
:value="resetPassword" :type="showResetPassword ? 'text' : 'password'"
                  placeholder="" class="password-input w-full px-4 py-2.5 pr-10 rounded-xl outline-none transition-all font-mono text-sm"
                  style="border: 1.5px solid var(--border); background: var(--bg-app); color: var(--text-primary);"
                  @input="emit('update:resetPassword', ($event.target as HTMLInputElement).value)" />
                <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 transition-opacity" :style="{ opacity: showResetPassword ? '1' : '0.4', color: 'var(--text-muted)' }" tabindex="-1" @click="emit('update:showResetPassword', !showResetPassword)">
                  <svg v-if="!showResetPassword" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                </button>
              </div>
            </div>

            <!-- Countdown progress bar -->
            <div class="space-y-1.5">
              <div class="h-1 rounded-full overflow-hidden" style="background: var(--border);">
                <div class="h-full rounded-full transition-all duration-1000" :style="{ width: ((resetCountdownTotal - resetCountdown) / resetCountdownTotal * 100) + '%', background: resetCountdown > 0 ? 'var(--border-strong)' : '#EF4444' }"></div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="px-6 pb-6 flex gap-3">
            <button class="flex-1 py-2.5 rounded-xl text-sm font-semibold transition-all" style="background: var(--bg-hover); color: var(--text-secondary);" @click="emit('close')">
              {{ t.libResetModalCancel }}
            </button>
            <button :disabled="resetCountdown > 0 || resetExecuting" class="flex-1 py-2.5 rounded-xl text-sm font-bold transition-all disabled:opacity-40 disabled:cursor-not-allowed" :style="resetCountdown === 0 && !resetExecuting ? 'background: var(--color-danger); color: white;' : 'background: var(--border); color: var(--text-muted);'" @click="emit('confirm')">
              <span v-if="resetExecuting">{{ t.libResetting }}</span>
              <span v-else>{{ t.libResetModalConfirmBtn(resetCountdown) }}</span>
            </button>
          </div>
          <div v-if="resetError" class="px-6 pb-4 text-center ts-xs" style="color: var(--color-danger);">{{ t.libError }}</div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: boolean
  lang: string
  t: Record<string, any>
  resetCountdownTotal: number
  isKeylessModeActive: boolean
  resetIsHardware: boolean
  resetCountdown: number
  resetExecuting: boolean
  resetError: boolean
  resetPassword: string
  showResetPassword: boolean
}>()

const emit = defineEmits<{
  'close': []
  'confirm': []
  'update:resetPassword': [value: string]
  'update:showResetPassword': [value: boolean]
}>()
</script>
