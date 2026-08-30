// The whole of the page's behaviour. Two things, and the page works without
// either: the tab swap is CSS-only via :has(), and the command is selectable
// text whether or not the copy button ever runs.

// The hero shows a macOS arm64 filename because the page has to show one real
// command, not a placeholder with angle brackets in it. Correct it to the
// visitor's platform where the browser will say, and leave it alone otherwise --
// the "All builds & checksums" link covers everyone else.
;(function nameTheRightTarball() {
  const el = document.querySelector('[data-cmd="binary"]')
  if (!el) return

  const ua = navigator.userAgent
  const platform = navigator.userAgentData?.platform ?? navigator.platform ?? ''
  const arm = /arm|aarch64/i.test(ua) || navigator.userAgentData?.architecture === 'arm'

  let os = null
  if (/Mac|Darwin/i.test(platform + ua)) os = 'darwin'
  else if (/Linux|X11/i.test(platform + ua) && !/Android/i.test(ua)) os = 'linux'
  else if (/Win/i.test(platform + ua)) os = 'windows'
  if (!os || os === 'windows') return // Windows ships a .zip; leave the default.

  // Apple silicon does not admit to being arm in the UA, so treat a modern Mac
  // as arm64 only when the hint agrees; otherwise keep the default.
  const arch = os === 'darwin' ? (arm || navigator.maxTouchPoints > 0 ? 'arm64' : 'arm64') : (arm ? 'arm64' : 'amd64')

  const next = el.dataset.template.replace('{os}', os).replace('{arch}', arch)
  el.dataset.copy = next
  el.querySelector('.text').textContent = next
})()

// Copy the command, never the prompt.
document.querySelectorAll('.copy').forEach((button) => {
  button.addEventListener('click', async () => {
    const strip = button.closest('.strip')
    const active = strip.querySelector('.cmd:not([hidden])[data-copy], .cmd[data-copy]')
    const shown = [...strip.querySelectorAll('.cmd')].find((c) => c.offsetParent !== null) || active
    const payload = shown?.dataset.copy
    if (!payload) return
    try {
      await navigator.clipboard.writeText(payload)
      const label = button.querySelector('.label')
      const was = label.textContent
      label.textContent = 'Copied'
      setTimeout(() => { label.textContent = was }, 1600)
    } catch {
      // A failed write should leave the label alone rather than lie about it.
    }
  })
})
