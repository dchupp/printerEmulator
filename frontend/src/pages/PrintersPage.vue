<template>
  <q-page padding class="page-narrow">
    <div class="row items-end q-mb-md">
      <div>
        <div class="text-h5 text-weight-medium">Network printers</div>
        <div class="text-body2 text-grey-7">Printers that jobs can be forwarded or relayed to. Zebra printers receive the raw ZPL; IPP printers receive the rendered label as a PDF.</div>
      </div>
      <q-space />
      <q-btn unelevated no-caps color="primary" icon="add" label="Add printer" @click="edit()" />
    </div>

    <q-card v-if="s.mode === 'forward'" flat bordered class="q-mb-md mode-card">
      <q-card-section class="row items-center q-gutter-sm">
        <q-icon name="call_made" size="24px" color="primary" />
        <div class="col">
          <div class="text-weight-medium">Forward mode is on</div>
          <div class="text-caption text-grey-7">Every job received on port {{ s.port }} goes to the printer marked <b>Forwarding</b>.</div>
        </div>
        <q-toggle :model-value="s.previewWhileForwarding" label="Also show labels" @update:model-value="v => updateSettings({ previewWhileForwarding: v }, 0)" />
      </q-card-section>
    </q-card>

    <div v-if="!state.printers.length" class="empty-state column items-center justify-center text-center q-py-xl">
      <div class="empty-art"><q-icon name="print" size="48px" /></div>
      <div class="text-h6 q-mt-md">No printers yet</div>
      <div class="text-body2 text-grey-7">Add a Zebra or IPP printer on your network to forward or relay jobs to it.</div>
      <q-btn class="q-mt-lg" unelevated no-caps color="primary" icon="add" label="Add printer" @click="edit()" />
    </div>

    <div class="printer-grid">
      <q-card v-for="p in state.printers" :key="p.printerID" flat bordered class="printer-card" :class="{ selected: isForwardTarget(p) }">
        <q-card-section class="row items-start no-wrap">
          <q-avatar :color="p.printerType === 'IPP' ? 'secondary' : 'primary'" text-color="white" size="42px" :icon="p.printerType === 'IPP' ? 'print' : 'qr_code_2'" />
          <div class="col q-ml-md" style="min-width: 0">
            <div class="text-subtitle1 text-weight-medium ellipsis">{{ p.printerName }}</div>
            <div class="text-caption text-grey-7 ellipsis">
              {{ p.printerType === 'IPP' ? (p.useTLS ? 'ipps' : 'ipp') + '://' : 'raw://' }}{{ p.ipAddress }}:{{ p.printerPort }}{{ p.printerType === 'IPP' ? p.ippEndpoint : '' }}
            </div>
            <div class="row q-gutter-xs q-mt-xs">
              <q-chip dense square size="sm" :label="p.printerType === 'IPP' ? 'IPP · PDF' : 'Zebra · raw ZPL'" />
              <q-chip v-if="isForwardTarget(p)" dense square size="sm" color="primary" text-color="white" icon="call_made" label="Forwarding" />
              <q-chip v-if="reach[p.printerID]" dense square size="sm" :color="reach[p.printerID].ok ? 'positive' : 'negative'" text-color="white"
                :icon="reach[p.printerID].ok ? 'wifi' : 'wifi_off'" :label="reach[p.printerID].ok ? `${reach[p.printerID].ms} ms` : 'Unreachable'">
                <q-tooltip v-if="!reach[p.printerID].ok">{{ reach[p.printerID].error }}</q-tooltip>
              </q-chip>
            </div>
          </div>
        </q-card-section>
        <q-separator />
        <q-card-actions>
          <q-btn v-if="!isForwardTarget(p)" flat dense no-caps color="primary" icon="call_made" label="Forward here" @click="forwardTo(p)" />
          <q-btn v-else flat dense no-caps icon="visibility" label="Back to preview" @click="updateSettings({ mode: 'preview' }, 0)" />
          <q-space />
          <q-btn flat round dense icon="network_check" :loading="testing === p.printerID" @click="test(p)"><q-tooltip>Check connection</q-tooltip></q-btn>
          <q-btn flat round dense icon="edit" @click="edit(p)"><q-tooltip>Edit</q-tooltip></q-btn>
          <q-btn flat round dense icon="delete_outline" color="negative" @click="remove(p)"><q-tooltip>Delete</q-tooltip></q-btn>
        </q-card-actions>
      </q-card>
    </div>

    <!-- Add / edit -->
    <q-dialog v-model="dialog">
      <q-card style="width: 480px; max-width: 92vw">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ form.printerID ? 'Edit printer' : 'Add printer' }}</div>
          <q-space />
          <q-btn flat round dense icon="close" v-close-popup />
        </q-card-section>
        <q-form @submit.prevent="save">
          <q-card-section class="q-gutter-md q-pt-none">
            <q-btn-toggle v-model="form.printerType" spread unelevated no-caps toggle-color="primary" class="type-toggle"
              :options="[{ value: 'Zebra', label: 'Zebra (raw ZPL)', icon: 'qr_code_2' }, { value: 'IPP', label: 'IPP (any printer)', icon: 'print' }]"
              @update:model-value="onType" />
            <q-input v-model="form.printerName" outlined dense label="Name" autofocus :rules="[v => !!v?.trim() || 'Required']" />
            <div class="row q-col-gutter-sm">
              <q-input v-model="form.ipAddress" class="col-8" outlined dense label="IP address or host name" :rules="[v => !!v?.trim() || 'Required']" />
              <q-input v-model.number="form.printerPort" class="col-4" outlined dense type="number" label="Port" :rules="[v => (v > 0 && v < 65536) || '1–65535']" />
            </div>
            <template v-if="form.printerType === 'IPP'">
              <q-input v-model="form.ippEndpoint" outlined dense label="IPP path" hint="Usually /ipp/print" />
              <q-toggle v-model="form.useTLS" label="Use TLS (IPPS)" />
            </template>
          </q-card-section>
          <q-card-actions align="right" class="q-pa-md">
            <q-btn flat no-caps label="Check connection" icon="network_check" :loading="testing === 'form'" @click="test(form, 'form')" />
            <q-space />
            <q-btn flat no-caps label="Cancel" v-close-popup />
            <q-btn unelevated no-caps color="primary" type="submit" label="Save" :loading="saving" />
          </q-card-actions>
        </q-form>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, state, updateSettings, refreshPrinters, notifyError, notifyOK } from '../state'

const $q = useQuasar()
const s = computed(() => state.settings)
const dialog = ref(false)
const saving = ref(false)
const testing = ref(null)
const reach = reactive({})
const blank = () => ({ printerID: 0, printerName: '', ipAddress: '', printerPort: 9100, printerType: 'Zebra', ippEndpoint: '/ipp/print', useTLS: false })
const form = ref(blank())

function isForwardTarget (p) { return s.value.mode === 'forward' && s.value.forwardPrinterID === p.printerID }

function edit (p) {
  form.value = p ? { ...blank(), ...p } : blank()
  dialog.value = true
}

function onType (t) {
  if (t === 'IPP' && form.value.printerPort === 9100) form.value.printerPort = 631
  if (t === 'Zebra' && form.value.printerPort === 631) form.value.printerPort = 9100
}

async function save () {
  saving.value = true
  try {
    const p = await api.SavePrinter({ ...form.value, printerPort: Number(form.value.printerPort) || 0 })
    await refreshPrinters()
    dialog.value = false
    notifyOK(`Saved ${p.printerName}`)
  } catch (e) {
    notifyError(e)
  } finally {
    saving.value = false
  }
}

async function test (p, key = p.printerID) {
  testing.value = key
  try {
    const ms = await api.TestPrinter({ ...p, printerPort: Number(p.printerPort) || 0 })
    if (key !== 'form') reach[p.printerID] = { ok: true, ms }
    else notifyOK(`Connected in ${ms} ms`)
  } catch (e) {
    if (key !== 'form') reach[p.printerID] = { ok: false, error: e.message || String(e) }
    else notifyError(e, 'Unreachable')
  } finally {
    testing.value = null
  }
}

function forwardTo (p) {
  updateSettings({ mode: 'forward', forwardPrinterID: p.printerID }, 0)
  notifyOK(`Jobs now go to ${p.printerName}`)
}

function remove (p) {
  $q.dialog({
    title: `Delete ${p.printerName}?`,
    message: 'It is also removed from any relay group.',
    cancel: { flat: true, noCaps: true },
    ok: { label: 'Delete', color: 'negative', unelevated: true, noCaps: true }
  }).onOk(async () => {
    try {
      await api.DeletePrinter(p.printerID)
      await refreshPrinters()
    } catch (e) { notifyError(e) }
  })
}
</script>
