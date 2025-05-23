<template>
  <q-page class="q-pa-md">
    <q-card>
      <q-card-section>
        <div class="text-h5">ZPL to Network Printer Service</div>
        <q-separator class="q-my-sm" />
        <q-form @submit.prevent>
          <q-select v-model="selectedPrinter" :options="ippPrinterOptions" label="Select IPP Printer" emit-value
            map-options class="q-mb-md" />
          <div class="row q-gutter-md items-center">
            <q-btn @click="startService" color="positive" :disable="serviceRunning || !selectedPrinter"
              label="Start Service" />
            <q-btn @click="stopService" color="negative" :disable="!serviceRunning" label="Stop Service" />
            <q-toggle v-model="serviceRunning" label="Service Running" color="purple-5" disable />
          </div>
        </q-form>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { GetPrinters, StartPrinterServer, StopPrintServer, GetPrinterRunStatus, SetPrinterZPLToPrinterMode, SelectPrinter } from 'app/wailsjs/go/main/App'

const ippPrinterOptions = ref([])
const selectedPrinter = ref(null)
const serviceRunning = ref(false)

async function loadIPPPrinters() {
  const printers = await GetPrinters()
  ippPrinterOptions.value = printers
    .filter(p => p.printerType === 'IPP')
    .map(p => ({ label: p.printerName, value: p.printerID }))
}

async function startService() {
  if (selectedPrinter.value) {
    serviceRunning.value = true
    await StartPrinterServer()
    await checkServiceStatus()
  }
}
async function stopService() {
  serviceRunning.value = false
  await StopPrintServer()
  await checkServiceStatus()
}
async function checkServiceStatus() {
  serviceRunning.value = await GetPrinterRunStatus()
}

onMounted(async () => {
  await SetPrinterZPLToPrinterMode()
  await loadIPPPrinters()
  await checkServiceStatus()
})
watch(selectedPrinter, async (newVal) => {
  if (newVal) {
    await SelectPrinter(newVal)
  }
})
</script>
