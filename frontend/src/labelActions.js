import { api, notifyError, notifyOK } from './state'

// Actions shared by the label card, its menus and the viewer.
export async function copyLabelImage (label) {
  try {
    const blob = await (await fetch(label.url)).blob()
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    notifyOK('Label image copied')
  } catch (e) {
    notifyError(e, 'Copy failed')
  }
}

export async function copyLabelZPL (label) {
  try {
    await navigator.clipboard.writeText(await api.JobZPL(label.jobID))
    notifyOK('ZPL copied')
  } catch (e) {
    notifyError(e, 'Copy failed')
  }
}

export async function saveLabelAs (label) {
  try {
    const path = await api.SaveLabelAs(label.id)
    if (path) notifyOK(`Saved ${path}`)
  } catch (e) {
    notifyError(e, 'Save failed')
  }
}

export async function sendLabelTo (label, printer) {
  try {
    const d = await api.ResendJob(label.jobID, printer.printerID)
    notifyOK(`Sent to ${d.printerName} (${d.millis} ms)`)
  } catch (e) {
    notifyError(e, printer.printerName)
  }
}
