import { ref, type Ref } from 'vue'

export interface ReadOnlyModeState {
  isReadOnly: Ref<boolean>
  isTouchDevice: boolean
  toggleReadOnly: () => void
  resetToDeviceDefault: () => void
}

export function useReadOnlyMode(): ReadOnlyModeState {
  const isTouchDevice = matchMedia('(pointer: coarse)').matches
  const isReadOnly = ref(isTouchDevice)

  const toggleReadOnly = () => {
    isReadOnly.value = !isReadOnly.value
  }

  const resetToDeviceDefault = () => {
    isReadOnly.value = isTouchDevice
  }

  return { isReadOnly, isTouchDevice, toggleReadOnly, resetToDeviceDefault }
}
