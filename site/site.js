// The whole of the page's behaviour, and the page works without any of it: the
// platform tabs swap with CSS :has(), and every command is selectable text
// whether or not the copy button ever runs.

// Preselect the tab matching the visitor's operating system.
//
// Only the OS is guessed here, never the architecture. A browser cannot tell an
// Apple Silicon Mac from an Intel one with any reliability -- Safari does not
// expose it at all -- and guessing wrong would hand someone a download that
// 404s. So the commands resolve the architecture in the shell with `uname -m`,
// where the answer is certain, and this only decides which tab opens first.
// Being wrong here costs a click.
;(function preselectPlatform() {
  const ua = navigator.userAgent
  const platform = navigator.userAgentData?.platform ?? navigator.platform ?? ''
  const both = platform + ' ' + ua

  let id = null
  if (/Mac|Darwin|iPhone|iPad/i.test(both)) id = 't-macos'
  else if (/Win/i.test(both)) id = 't-windows'
  else if (/Linux|X11|Android|CrOS/i.test(both)) id = 't-linux'
  if (!id) return

  const radio = document.getElementById(id)
  if (radio) radio.checked = true
})()

// Copy the command that is currently showing, never the prompt in front of it.
document.querySelectorAll('.copy').forEach((button) => {
  button.addEventListener('click', async () => {
    const strip = button.closest('.strip')
    const visible = [...strip.querySelectorAll('.cmd')].find((c) => c.offsetParent !== null)
    const payload = visible?.dataset.copy
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
