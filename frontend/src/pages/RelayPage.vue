<template>
  <q-page padding class="page-narrow">
    <div class="row items-end q-mb-md">
      <div>
        <div class="text-h5 text-weight-medium">Relay groups</div>
        <div class="text-body2 text-grey-7">In relay mode every job is sent to all printers of the active group at the same time.</div>
      </div>
      <q-space />
      <q-btn unelevated no-caps color="primary" icon="add" label="New group" :disable="!state.printers.length" @click="edit()" />
    </div>

    <q-banner v-if="!state.printers.length" rounded class="q-mb-md mode-banner">
      <template #avatar><q-icon name="info" color="primary" /></template>
      Add network printers first, then group them here.
      <template #action><q-btn flat no-caps color="primary" label="Add printers" to="/printers" /></template>
    </q-banner>

    <q-card v-if="s.mode === 'relay'" flat bordered class="q-mb-md mode-card">
      <q-card-section class="row items-center q-gutter-sm">
        <q-icon name="device_hub" size="24px" color="primary" />
        <div class="col">
          <div class="text-weight-medium">Relay mode is on</div>
          <div class="text-caption text-grey-7">Jobs received on port {{ s.port }} go to every printer in the <b>Active</b> group.</div>
        </div>
        <q-toggle :model-value="s.previewWhileForwarding" label="Also show labels" @update:model-value="v => updateSettings({ previewWhileForwarding: v }, 0)" />
      </q-card-section>
    </q-card>

    <div class="printer-grid">
      <q-card v-for="g in state.groups" :key="g.groupID" flat bordered class="printer-card" :class="{ selected: isActive(g) }">
        <q-card-section>
          <div class="row items-center no-wrap">
            <q-avatar color="primary" text-color="white" size="42px" icon="device_hub" />
            <div class="col q-ml-md" style="min-width: 0">
              <div class="text-subtitle1 text-weight-medium ellipsis">{{ g.name }}</div>
              <div class="text-caption text-grey-7">{{ g.printerIDs.length }} printer{{ g.printerIDs.length === 1 ? '' : 's' }}</div>
            </div>
            <q-chip v-if="isActive(g)" dense square size="sm" color="primary" text-color="white" icon="bolt" label="Active" />
          </div>
          <div class="row q-gutter-xs q-mt-sm">
            <q-chip v-for="id in g.printerIDs" :key="id" dense size="sm" :icon="printerByID[id]?.printerType === 'IPP' ? 'print' : 'qr_code_2'">
              {{ printerByID[id]?.printerName || `#${id} (deleted)` }}
            </q-chip>
          </div>
        </q-card-section>
        <q-separator />
        <q-card-actions>
          <q-btn v-if="!isActive(g)" flat dense no-caps color="primary" icon="bolt" label="Relay to this group" @click="activate(g)" />
          <q-btn v-else flat dense no-caps icon="visibility" label="Back to preview" @click="updateSettings({ mode: 'preview' }, 0)" />
          <q-space />
          <q-btn flat round dense icon="edit" @click="edit(g)"><q-tooltip>Edit</q-tooltip></q-btn>
          <q-btn flat round dense icon="delete_outline" color="negative" @click="remove(g)"><q-tooltip>Delete</q-tooltip></q-btn>
        </q-card-actions>
      </q-card>
    </div>

    <q-dialog v-model="dialog">
      <q-card style="width: 480px; max-width: 92vw">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ form.groupID ? 'Edit group' : 'New relay group' }}</div>
          <q-space />
          <q-btn flat round dense icon="close" v-close-popup />
        </q-card-section>
        <q-form @submit.prevent="save">
          <q-card-section class="q-pt-none q-gutter-md">
            <q-input v-model="form.name" outlined dense label="Group name" autofocus placeholder="e.g. Shipping dock" />
            <div>
              <div class="text-caption text-grey-7 q-mb-xs">Printers</div>
              <q-list bordered separator class="rounded-borders">
                <q-item v-for="p in state.printers" :key="p.printerID" tag="label" v-ripple dense>
                  <q-item-section avatar><q-checkbox v-model="form.printerIDs" :val="p.printerID" /></q-item-section>
                  <q-item-section>
                    <q-item-label>{{ p.printerName }}</q-item-label>
                    <q-item-label caption>{{ p.printerType }} · {{ p.ipAddress }}:{{ p.printerPort }}</q-item-label>
                  </q-item-section>
                </q-item>
              </q-list>
            </div>
          </q-card-section>
          <q-card-actions align="right" class="q-pa-md">
            <q-btn flat no-caps label="Cancel" v-close-popup />
            <q-btn unelevated no-caps color="primary" type="submit" label="Save" :disable="!form.printerIDs.length" :loading="saving" />
          </q-card-actions>
        </q-form>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, state, printerByID, updateSettings, refreshPrinters, notifyError, notifyOK } from '../state'

const $q = useQuasar()
const s = computed(() => state.settings)
const dialog = ref(false)
const saving = ref(false)
const form = ref({ groupID: 0, name: '', printerIDs: [] })

function isActive (g) { return s.value.mode === 'relay' && s.value.relayGroupID === g.groupID }

function edit (g) {
  form.value = g ? { ...g, printerIDs: [...g.printerIDs] } : { groupID: 0, name: '', printerIDs: [] }
  dialog.value = true
}

async function save () {
  saving.value = true
  try {
    const g = await api.SaveRelayGroup(form.value)
    await refreshPrinters()
    dialog.value = false
    notifyOK(`Saved ${g.name}`)
  } catch (e) {
    notifyError(e)
  } finally {
    saving.value = false
  }
}

function activate (g) {
  updateSettings({ mode: 'relay', relayGroupID: g.groupID }, 0)
  notifyOK(`Jobs now go to every printer in ${g.name}`)
}

function remove (g) {
  $q.dialog({
    title: `Delete ${g.name}?`,
    cancel: { flat: true, noCaps: true },
    ok: { label: 'Delete', color: 'negative', unelevated: true, noCaps: true }
  }).onOk(async () => {
    try {
      await api.DeleteRelayGroup(g.groupID)
      await refreshPrinters()
    } catch (e) { notifyError(e) }
  })
}
</script>
