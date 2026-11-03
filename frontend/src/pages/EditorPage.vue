<template>
  <q-page class="editor-page row no-wrap">
    <div class="col column no-wrap editor-pane">
      <div class="row items-center q-px-md q-py-sm q-gutter-x-sm toolbar">
        <div class="text-subtitle1 text-weight-medium">ZPL Editor</div>
        <q-space />
        <q-btn-dropdown flat dense no-caps icon="library_books" label="Samples">
          <q-list dense style="min-width: 220px">
            <q-item v-for="smp in SAMPLES" :key="smp.name" clickable v-close-popup @click="load(smp.zpl)">
              <q-item-section>{{ smp.name }}</q-item-section>
            </q-item>
          </q-list>
        </q-btn-dropdown>
        <q-btn flat dense no-caps icon="file_open" label="Open file" @click="fileInput.click()" />
        <input ref="fileInput" type="file" accept=".zpl,.txt,.prn" class="hidden" @change="openFile" />
        <q-btn flat dense round icon="content_copy" @click="copy"><q-tooltip>Copy ZPL</q-tooltip></q-btn>
        <q-btn flat dense round icon="clear_all" @click="zpl = ''"><q-tooltip>Clear</q-tooltip></q-btn>
      </div>
      <q-separator />
      <textarea
        v-model="zpl"
        class="col zpl-input"
        spellcheck="false"
        autocomplete="off"
        placeholder="^XA&#10;^FO50,50^A0N,50^FDHello^FS&#10;^XZ"
        @keydown.ctrl.enter.prevent="send"
        @keydown.meta.enter.prevent="send"
      />
      <div class="row items-center q-px-md q-py-sm q-gutter-x-sm toolbar">
        <div class="text-caption text-grey-6">{{ zpl.length.toLocaleString() }} characters · Ctrl+Enter prints</div>
        <q-space />
        <q-btn outline no-caps color="primary" icon="receipt_long" label="Add to labels" :disable="!zpl.trim()" :loading="sending === 'preview'" @click="send(true)">
          <q-tooltip>Render into the label list (never forwarded)</q-tooltip>
        </q-btn>
        <q-btn unelevated no-caps color="primary" :icon="modeIcon" :label="printLabel" :disable="!zpl.trim()" :loading="sending === 'print'" @click="send(false)">
          <q-tooltip>Process exactly like a job received on the printer port</q-tooltip>
        </q-btn>
      </div>
    </div>

    <div class="preview-pane column no-wrap">
      <div class="row items-center q-px-md q-py-sm toolbar">
        <div class="text-subtitle2">Live preview</div>
        <q-spinner v-if="rendering" size="14px" color="primary" class="q-ml-sm" />
        <q-space />
        <div class="text-caption text-grey-6">
          {{ s.labelWidth }} × {{ s.labelHeight }} in · {{ s.dpmm }} dpmm<span v-if="preview.images.length"> · {{ preview.millis.toFixed(1) }} ms</span>
        </div>
      </div>
      <q-separator />
      <div class="col scroll q-pa-md preview-canvas">
        <div v-if="error" class="text-negative text-body2">{{ error }}</div>
        <div v-else-if="!preview.images.length" class="text-grey-6 text-body2 text-center q-mt-xl">
          {{ zpl.trim() ? 'No label: the ZPL needs ^XA … ^XZ.' : 'Type or paste ZPL to see it rendered.' }}
        </div>
        <div v-for="(img, i) in preview.images" :key="i" class="label-paper q-mb-md">
          <img :src="img" alt="Preview" />
        </div>
      </div>
      <template v-if="preview.warnings.length">
        <q-separator />
        <q-expansion-item dense dense-toggle icon="warning_amber" header-class="text-warning" :label="`${preview.warnings.length} warning${preview.warnings.length > 1 ? 's' : ''}`">
          <div class="q-px-md q-pb-sm warnings-box">
            <div v-for="(w, i) in preview.warnings" :key="i" class="warning-line">{{ w }}</div>
          </div>
        </q-expansion-item>
      </template>
    </div>
  </q-page>
</template>

<script setup>
import { computed, reactive, ref, watch, onActivated } from 'vue'
import { useRoute } from 'vue-router'
import { api, state, MODES, notifyError, notifyOK } from '../state'

defineOptions({ name: 'EditorPage' })

const SAMPLES = [
  {
    name: 'Shipping label (4×6)',
    zpl: `^XA
^CF0,60
^FO50,50^GB100,100,100^FS
^FO75,75^FR^GB100,100,100^FS
^FO93,93^GB40,40,40^FS
^FO220,50^FDIntershipping, Inc.^FS
^CF0,30
^FO220,115^FD1000 Shipping Lane^FS
^FO220,155^FDShelbyville TN 38102^FS
^FO220,195^FDUnited States (USA)^FS
^FO50,250^GB700,3,3^FS
^CFA,30
^FO50,300^FDJohn Doe^FS
^FO50,340^FD100 Main Street^FS
^FO50,380^FDSpringfield TN 39021^FS
^FO50,420^FDUnited States (USA)^FS
^CFA,15
^FO600,300^GB150,150,3^FS
^FO638,340^FDPermit^FS
^FO638,390^FD123456^FS
^FO50,500^GB700,3,3^FS
^BY5,2,270
^FO100,550^BC^FD12345678^FS
^FO50,900^GB700,250,3^FS
^FO400,900^GB3,250,3^FS
^CF0,40
^FO100,960^FDCtr. X34B-1^FS
^FO100,1010^FDREF1 F00B47^FS
^FO100,1060^FDREF2 BL4H8^FS
^CF0,190
^FO470,955^FDCA^FS
^XZ`
  },
  {
    name: 'Barcodes',
    zpl: `^XA
^FO40,40^A0N,36^FDCode 128^FS
^FO40,90^BY3^BCN,120,Y,N,N^FD>;123456789012^FS
^FO40,290^A0N,36^FDQR Code^FS
^FO40,340^BQN,2,8^FDQA,https://github.com/dchupp/printerEmulator^FS
^FO420,290^A0N,36^FDData Matrix^FS
^FO420,350^BXN,10,200^FDDataMagik^FS
^FO40,640^A0N,36^FDPDF417^FS
^FO40,690^B7N,8,5,,,N^FDPrinter Emulator 3^FS
^XZ`
  },
  {
    name: 'Product tag (2×1)',
    zpl: `^XA
^PW406^LL203
^FO20,20^A0N,40^FDWidget Pro^FS
^FO20,70^A0N,26^FDSKU 44-1092-B^FS
^FO20,110^BY2^BCN,60,N,N,N^FD441092B^FS
^FO300,20^GB86,86,4^FS
^FO316,44^A0N,40^FD$9^FS
^XZ`
  },
  {
    name: 'Stored format (^DF / ^XF)',
    zpl: `^XA
^DFR:BIN.ZPL^FS
^FO40,40^A0N,60^FN1^FS
^FO40,120^BY3^BCN,100,Y,N,N^FN2^FS
^XZ
^XA
^XFR:BIN.ZPL^FS
^FN1^FDBin A-14^FS
^FN2^FDA14^FS
^XZ`
  }
]

const route = useRoute()
const s = computed(() => state.settings)
const zpl = ref(readDraft())
const preview = reactive({ images: [], warnings: [], millis: 0 })
const error = ref('')
const rendering = ref(false)
const sending = ref('')
const fileInput = ref(null)

function readDraft () { try { return localStorage.getItem('editor.zpl') ?? SAMPLES[0].zpl } catch { return SAMPLES[0].zpl } }

const mode = computed(() => MODES.find(m => m.value === s.value.mode) || MODES[0])
const modeIcon = computed(() => s.value.mode === 'preview' ? 'print' : mode.value.icon)
const printLabel = computed(() => ({ preview: 'Print', forward: 'Print & forward', relay: 'Print & relay' }[s.value.mode]))

// Render as the user types, at most one request in flight.
let timer = null
let running = false
let again = false
async function render () {
  if (running) { again = true; return }
  running = true
  rendering.value = true
  try {
    const src = zpl.value
    if (!src.trim()) {
      Object.assign(preview, { images: [], warnings: [], millis: 0 })
      error.value = ''
    } else {
      const r = await api.PreviewZPL(src)
      Object.assign(preview, { images: r.images || [], warnings: r.warnings || [], millis: r.millis })
      error.value = ''
    }
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    running = false
    rendering.value = false
    if (again) { again = false; render() }
  }
}
function schedule () {
  clearTimeout(timer)
  timer = setTimeout(render, 250)
}
watch(zpl, v => {
  try { localStorage.setItem('editor.zpl', v) } catch {}
  schedule()
})
watch(() => [s.value.labelWidth, s.value.labelHeight, s.value.dpmm, s.value.rotation], schedule)
render()

// "Open in ZPL editor" from a label passes ?job=<id>.
async function loadJob (id) {
  if (!id) return
  try {
    zpl.value = await api.JobZPL(Number(id))
  } catch (e) {
    notifyError(e)
  }
}
watch(() => route.query.job, loadJob)
onActivated(() => loadJob(route.query.job))

function load (text) { zpl.value = text }

async function openFile (e) {
  const f = e.target.files?.[0]
  if (f) zpl.value = await f.text()
  e.target.value = ''
}

async function copy () {
  try {
    await navigator.clipboard.writeText(zpl.value)
    notifyOK('ZPL copied')
  } catch (e) { notifyError(e) }
}

async function send (previewOnly = false) {
  if (!zpl.value.trim()) return
  sending.value = previewOnly ? 'preview' : 'print'
  try {
    const job = await api.PrintZPL(zpl.value, previewOnly)
    const failed = (job.deliveries || []).filter(d => !d.ok)
    if (!job.error && !failed.length) {
      const parts = []
      if (job.labels) parts.push(`${job.labels} label${job.labels > 1 ? 's' : ''} rendered`)
      if (job.deliveries?.length) parts.push(`sent to ${job.deliveries.map(d => d.printerName).join(', ')}`)
      notifyOK(parts.join(', ') || 'Done')
    }
  } catch (e) {
    notifyError(e)
  } finally {
    sending.value = ''
  }
}
</script>
