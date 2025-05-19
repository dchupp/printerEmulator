<template>
  <q-page class="q-pa-md">
    <q-card>
      <q-card-section>
        <div class="text-h5">Network Printers</div>
        <q-separator class="q-my-sm" />
        <q-form @submit.prevent="onAddPrinter">
          <div class="row q-col-gutter-md">
            <q-input v-model="form.printerName" label="Printer Name" class="col" dense outlined required />
            <q-input v-model="form.ipAddress" label="IP Address" class="col" dense outlined required />
            <q-select v-model="form.printerType" :options="printerTypeOptions" label="Type" class="col" dense outlined
              required />
            <q-btn type="submit" color="primary" label="Add Printer" class="q-ml-md" />
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
        <q-card>
          <q-card-section>
            <div class="text-h6">Edit Printer</div>
            <q-form @submit.prevent="onUpdatePrinter">
              <q-input v-model="editForm.printerName" label="Printer Name" dense outlined required />
              <q-input v-model="editForm.ipAddress" label="IP Address" dense outlined required />
              <q-select v-model="editForm.printerType" :options="printerTypeOptions" label="Type" dense outlined
                required />
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
const form = ref({ printerName: '', ipAddress: '', printerType: '' })
const editDialog = ref(false)
const editForm = ref({ printerID: null, printerName: '', ipAddress: '', printerType: '' })

async function loadPrinters() {
  printers.value = await GetPrinters()
}
onMounted(loadPrinters)

async function onAddPrinter() {
  // Ensure we pass a string value for printerType
  await AddPrinter({
    printerName: form.value.printerName,
    ipAddress: form.value.ipAddress,
    printerType: typeof form.value.printerType === 'object' && form.value.printerType.value ? form.value.printerType.value : form.value.printerType
  })
  form.value = { printerName: '', ipAddress: '', printerType: '' }
  await loadPrinters()
}
function editPrinter(printer) {
  // Set editForm.printerType to the string value for the select
  editForm.value = {
    ...printer,
    printerType: printer.printerType
  }
  editDialog.value = true
}
async function onUpdatePrinter() {
  // Ensure we pass a string value for printerType
  await UpdatePrinter({
    printerID: editForm.value.printerID,
    printerName: editForm.value.printerName,
    ipAddress: editForm.value.ipAddress,
    printerType: typeof editForm.value.printerType === 'object' && editForm.value.printerType.value ? editForm.value.printerType.value : editForm.value.printerType
  })
  editDialog.value = false
  await loadPrinters()
}
async function deletePrinter(printerID) {
  await DeletePrinter(printerID)
  await loadPrinters()
}
</script>
