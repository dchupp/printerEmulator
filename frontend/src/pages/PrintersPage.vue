<template>
  <q-page class="q-pa-md">
    <q-card>
      <q-card-section>
        <div class="text-h5">Network Printers</div>
        <q-separator class="q-my-sm" />
        <q-form @submit.prevent="onAddPrinter">
          <div class="row q-col-gutter-md items-center">
            <q-input v-model="form.printerName" label="Printer Name" class="col" dense outlined required />
            <q-input v-model="form.ipAddress" label="IP Address" class="col" dense outlined required />
            <q-input v-model.number="form.printerPort" label="Port" class="col-2" dense outlined type="number" />
            <q-select v-model="form.printerType" :options="printerTypeOptions" label="Type" class="col-2" dense outlined
              required />
          </div>
          <div v-if="getTypeValue(form.printerType) === 'IPP'" class="row q-col-gutter-md q-mt-sm items-center">
            <q-input v-model="form.ippEndpoint" label="IPP Endpoint" class="col" dense outlined placeholder="/ipp/print" />
            <q-toggle v-model="form.useTLS" label="Use TLS (IPPS)" class="col-3" />
          </div>
          <div class="row q-mt-sm">
            <q-btn type="submit" color="primary" label="Add Printer" />
          </div>
        </q-form>
      </q-card-section>
      <q-separator />
      <q-card-section>
        <q-table :rows="printers" :columns="columns" row-key="printerID" flat>
          <template v-slot:body-cell-actions="props">
            <q-btn size="sm" color="primary" icon="edit" flat @click="editPrinter(props.row)" />
            <q-btn size="sm" color="negative" icon="delete" flat @click="deletePrinter(props.row.printerID)" />
          </template>
        </q-table>
      </q-card-section>
      <q-dialog v-model="editDialog">
        <q-card style="min-width: 400px">
          <q-card-section>
            <div class="text-h6">Edit Printer</div>
            <q-form @submit.prevent="onUpdatePrinter">
              <q-input v-model="editForm.printerName" label="Printer Name" dense outlined required class="q-mb-sm" />
              <q-input v-model="editForm.ipAddress" label="IP Address" dense outlined required class="q-mb-sm" />
              <q-input v-model.number="editForm.printerPort" label="Port" dense outlined type="number" class="q-mb-sm" />
              <q-select v-model="editForm.printerType" :options="printerTypeOptions" label="Type" dense outlined
                required class="q-mb-sm" />
              <div v-if="getTypeValue(editForm.printerType) === 'IPP'">
                <q-input v-model="editForm.ippEndpoint" label="IPP Endpoint" dense outlined placeholder="/ipp/print" class="q-mb-sm" />
                <q-toggle v-model="editForm.useTLS" label="Use TLS (IPPS)" />
              </div>
              <div class="q-mt-md">
                <q-btn type="submit" color="primary" label="Save" />
                <q-btn flat label="Cancel" color="grey" @click="editDialog = false" />
              </div>
            </q-form>
          </q-card-section>
        </q-card>
      </q-dialog>
    </q-card>
    <div class="q-mt-lg flex flex-center">
      <q-btn to="/" color="secondary" label="Back to Home" icon="home" />
    </div>
  </q-page>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { GetPrinters, AddPrinter, UpdatePrinter, DeletePrinter } from 'app/wailsjs/go/main/App'

const printers = ref([])
const columns = [
  { name: 'printerName', label: 'Name', field: 'printerName', align: 'left' },
  { name: 'ipAddress', label: 'IP Address', field: 'ipAddress', align: 'left' },
  { name: 'printerPort', label: 'Port', field: 'printerPort', align: 'left' },
  { name: 'printerType', label: 'Type', field: 'printerType', align: 'left' },
  { name: 'actions', label: 'Actions', field: 'actions', align: 'right' }
]
const printerTypeOptions = [
  { label: 'IPP', value: 'IPP' },
  { label: 'Zebra', value: 'Zebra' }
]
const form = ref({ printerName: '', ipAddress: '', printerPort: 9100, printerType: '', ippEndpoint: '/ipp/print', useTLS: false })
const editDialog = ref(false)
const editForm = ref({ printerID: null, printerName: '', ipAddress: '', printerPort: 9100, printerType: '', ippEndpoint: '/ipp/print', useTLS: false })

// Helper to get the string value from printerType (handles both object and string)
function getTypeValue(type) {
  return typeof type === 'object' && type?.value ? type.value : type
}

async function loadPrinters() {
  printers.value = await GetPrinters()
}
onMounted(loadPrinters)

async function onAddPrinter() {
  const typeValue = getTypeValue(form.value.printerType)
  await AddPrinter({
    printerName: form.value.printerName,
    ipAddress: form.value.ipAddress,
    printerPort: form.value.printerPort || (typeValue === 'IPP' ? 631 : 9100),
    printerType: typeValue,
    ippEndpoint: form.value.ippEndpoint || '/ipp/print',
    useTLS: form.value.useTLS || false
  })
  form.value = { printerName: '', ipAddress: '', printerPort: 9100, printerType: '', ippEndpoint: '/ipp/print', useTLS: false }
  await loadPrinters()
}
function editPrinter(printer) {
  editForm.value = {
    ...printer,
    printerType: printer.printerType,
    ippEndpoint: printer.ippEndpoint || '/ipp/print',
    useTLS: printer.useTLS || false
  }
  editDialog.value = true
}
async function onUpdatePrinter() {
  const typeValue = getTypeValue(editForm.value.printerType)
  await UpdatePrinter({
    printerID: editForm.value.printerID,
    printerName: editForm.value.printerName,
    ipAddress: editForm.value.ipAddress,
    printerPort: editForm.value.printerPort || (typeValue === 'IPP' ? 631 : 9100),
    printerType: typeValue,
    ippEndpoint: editForm.value.ippEndpoint || '/ipp/print',
    useTLS: editForm.value.useTLS || false
  })
  editDialog.value = false
  await loadPrinters()
}
async function deletePrinter(printerID) {
  await DeletePrinter(printerID)
  await loadPrinters()
}
</script>
