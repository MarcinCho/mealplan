import { defineStore } from 'pinia'

// One short confirmation at a time ("Zapisano przepis „Zupa”"), shown by the app shell so it
// survives navigating away from the screen that triggered it.
export const useNotice = defineStore('notice', {
  state: () => ({ text: '', open: false, seq: 0 }),
  actions: {
    show(text: string) {
      this.text = text
      this.open = true
      this.seq++ // a new notice restarts the timer even when the text repeats
    },
  },
})
