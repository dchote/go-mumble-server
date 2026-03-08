// Mumble permission bitmask values (from pkg/mumble/permission.go)
export const PERMISSIONS = [
  { name: 'Write', value: 0x1, desc: 'Full control (implies all)' },
  { name: 'Traverse', value: 0x2, desc: 'Pass through channel' },
  { name: 'Enter', value: 0x4, desc: 'Enter channel' },
  { name: 'Speak', value: 0x8, desc: 'Transmit voice' },
  { name: 'MuteDeafen', value: 0x10, desc: 'Mute/deafen others' },
  { name: 'Move', value: 0x20, desc: 'Move users' },
  { name: 'MakeChannel', value: 0x40, desc: 'Create permanent channels' },
  { name: 'LinkChannel', value: 0x80, desc: 'Link channels' },
  { name: 'Whisper', value: 0x100, desc: 'Whisper to channel' },
  { name: 'TextMessage', value: 0x200, desc: 'Send text messages' },
  { name: 'MakeTempChannel', value: 0x400, desc: 'Create temporary channels' },
  { name: 'Listen', value: 0x800, desc: 'Listen without joining' },
  { name: 'Kick', value: 0x10000, desc: 'Kick users (root)' },
  { name: 'Ban', value: 0x20000, desc: 'Ban users (root)' },
  { name: 'Register', value: 0x40000, desc: 'Register users (root)' },
  { name: 'SelfRegister', value: 0x80000, desc: 'Self-register (root)' },
  { name: 'ResetUser', value: 0x100000, desc: 'Reset user content (root)' },
]

export function hasPermission(mask, permValue) {
  if (mask & 0x1) return true // Write implies all
  return (mask & permValue) === permValue
}

export function setPermission(mask, permValue, grant) {
  if (grant) return mask | permValue
  return mask & ~permValue
}
