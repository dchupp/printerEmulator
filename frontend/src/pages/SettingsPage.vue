<template>
  <q-page padding class="page-narrow settings-page">
    <div class="text-h5 text-weight-medium q-mb-md">Settings</div>

    <q-card flat bordered class="q-mb-md">
      <q-card-section>
        <div class="section-title"><q-icon name="receipt_long" /> Labels</div>
        <div class="row q-col-gutter-md">
          <q-input class="col-6 col-sm-3" dense outlined type="number" step="0.25" label="Width (in)" :model-value="s.labelWidth" @update:model-value="v => num('labelWidth', v)" />
          <q-input class="col-6 col-sm-3" dense outlined type="number" step="0.25" label="Height (in)" :model-value="s.labelHeight" @update:model-value="v => num('labelHeight', v)" />
          <q-select class="col-12 col-sm-6" dense outlined emit-value map-options label="Print density" :options="DPMM_OPTIONS" :model-value="s.dpmm" @update:model-value="v => updateSettings({ dpmm: v }, 0)" />
        </div>
        <div class="row items-center q-mt-md q-gutter-x-md">
          <div class="text-body2">Rotation</div>
          <q-btn-toggle dense unelevated no-caps toggle-color="primary" class="rot-toggle" :options="ROTATIONS.map(r => ({ value: r, label: r + '°' }))" :model-value="s.rotation" @update:model-value="v => updateSettings({ rotation: v }, 0)" />
        </div>
      </q-card-section>
      <q-list>
        <SettingToggle field="expandCopies" label="Print every ^PQ copy" caption="Show each copy and serial number instead of one image per format" />
        <q-item>
          <q-item-section>
            <q-item-label>Labels kept in the list</q-item-label>
            <q-item-label caption>Older labels are dropped from memory (files on disk are kept)</q-item-label>
          </q-item-section>
          <q-item-section side>
            <q-select dense outlined emit-value map-options style="width: 110px" :options="[50, 100, 200, 500, 1000].map(n => ({ value: n, label: String(n) }))" :model-value="s.historyLimit" @update:model-value="v => updateSettings({ historyLimit: v }, 0)" />
          </q-item-section>
        </q-item>
        <q-item>
          <q-item-section>
            <q-item-label>Printer memory</q-item-label>
            <q-item-label caption>Forget graphics, formats and fonts stored by earlier jobs, like power-cycling the printer</q-item-label>
          </q-item-section>
          <q-item-section side>
            <q-btn outline no-caps dense class="q-px-sm" label="Reset" icon="restart_alt" @click="resetMemory" />
          </q-item-section>
        </q-item>
      </q-list>
    </q-card>

    <q-card flat bordered class="q-mb-md">
      <q-card-section>
        <div class="section-title"><q-icon name="lan" /> Printer port</div>
        <div class="row q-col-gutter-md items-center">
          <q-input class="col-6 col-sm-3" dense outlined type="number" label="TCP port" :model-value="s.port" @update:model-value="v => num('port', v, 800)" />
          <div class="col text-caption text-grey-7">
            Install a Zebra ZPL driver pointing at <code>127.0.0.1:{{ s.port }}</code>. The printer restarts automatically when the port changes.
          </div>
        </div>
      </q-card-section>
      <q-list>
        <SettingToggle field="listenAll" label="Accept jobs from other computers" caption="Listen on every network interface instead of only this computer" />
        <SettingToggle field="autoStartServer" label="Start the printer when the app opens" />
        <q-item tag="label">
          <q-item-section>
            <q-item-label>Open at login</q-item-label>
            <q-item-label caption>Launch the emulator when you sign in</q-item-label>
          </q-item-section>
          <q-item-section side><q-toggle :model-value="state.autoStart" @update:model-value="setAutoStart" /></q-item-section>
        </q-item>
        <SettingToggle field="closeToTray" label="Keep running in the tray when the window closes" caption="Quit from the tray icon menu" />
      </q-list>
    </q-card>

    <q-card flat bordered class="q-mb-md">
      <q-card-section>
        <div class="section-title"><q-icon name="save" /> Save to disk</div>
        <div class="row items-center q-gutter-sm no-wrap">
          <q-input class="col" dense outlined readonly :model-value="s.saveDir" placeholder="No folder chosen" label="Folder">
            <template v-if="s.saveDir" #append><q-icon name="close" class="cursor-pointer" @click="updateSettings({ saveDir: '', saveLabels: false }, 0)" /></template>
          </q-input>
          <q-btn outline no-caps icon="folder_open" label="Choose…" @click="chooseDir" />
          <q-btn flat round icon="open_in_new" :disable="!s.saveDir" @click="openDir"><q-tooltip>Show in file manager</q-tooltip></q-btn>
        </div>
      </q-card-section>
      <q-list>
        <SettingToggle field="saveLabels" label="Save every label as PNG" :disable="!s.saveDir" />
        <SettingToggle field="saveZPL" label="Also save the job's ZPL" :disable="!s.saveDir || !s.saveLabels" />
      </q-list>
    </q-card>

    <q-card flat bordered class="q-mb-md">
      <q-card-section>
        <div class="section-title"><q-icon name="palette" /> Appearance</div>
        <q-btn-toggle unelevated no-caps toggle-color="primary" :model-value="s.theme"
          :options="[{ value: 'auto', label: 'System', icon: 'brightness_auto' }, { value: 'light', label: 'Light', icon: 'light_mode' }, { value: 'dark', label: 'Dark', icon: 'dark_mode' }]"
          @update:model-value="v => updateSettings({ theme: v }, 0)" />
      </q-card-section>
    </q-card>

    <div class="text-caption text-grey-6 text-center q-my-lg">
      ZPL Printer Emulator v{{ state.version }} · labels rendered on this computer with DataMagik ZPL-To-Image · © Chupp Consulting, LLC
    </div>
  </q-page>
</template>

<script setup>
import { computed, h } from 'vue'
import { useQuasar, QItem, QItemSection, QItemLabel, QToggle } from 'quasar'
import { api, state, updateSettings, DPMM_OPTIONS, ROTATIONS, notifyError, notifyOK } from '../state'

const $q = useQuasar()
const s = computed(() => state.settings)

// A list row with a switch bound to one boolean setting.
const SettingToggle = {
  props: { field: String, label: String, caption: String, disable: Boolean },
  setup (props) {
    return () => h(QItem, { tag: 'label', disable: props.disable }, () => [
      h(QItemSection, null, () => [
        h(QItemLabel, null, () => props.label),
        props.caption ? h(QItemLabel, { caption: true }, () => props.caption) : null
      ]),
      h(QItemSection, { side: true }, () => h(QToggle, {
        modelValue: state.settings[props.field],
        disable: props.disable,
        'onUpdate:modelValue': v => updateSettings({ [props.field]: v }, 0)
      }))
    ])
  }
}

function num (key, v, delay) {
  const n = key === 'port' ? parseInt(v, 10) : parseFloat(v)
  if (n > 0) updateSettings({ [key]: n }, delay)
}

async function chooseDir () {
  try {
    const dir = await api.ChooseSaveDirectory()
    if (dir) {
      state.settings.saveDir = dir
      updateSettings({ saveLabels: true }, 0)
    }
  } catch (e) { notifyError(e) }
}

async function openDir () {
  try { await api.OpenSaveDirectory() } catch (e) { notifyError(e) }
}

async function setAutoStart (v) {
  try {
    await api.SetAutoStart(v)
    state.autoStart = await api.AutoStartEnabled()
  } catch (e) { notifyError(e, 'Open at login') }
}

function resetMemory () {
  $q.dialog({
    title: 'Reset printer memory?',
    message: 'Stored graphics (~DG), formats (^DF) and downloaded fonts are forgotten.',
    cancel: { flat: true, noCaps: true },
    ok: { label: 'Reset', color: 'primary', unelevated: true, noCaps: true }
  }).onOk(async () => {
    await api.ResetPrinterMemory()
    notifyOK('Printer memory cleared')
  })
}
</script>
