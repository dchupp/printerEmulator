<template>
  <q-page class="q-pa-md">
    <q-card>
      <q-card-section>
        <div class="text-h5">Printer Relay Setup</div>
        <q-separator class="q-my-sm" />
        <q-form @submit.prevent="addRelay">
          <q-select v-model="selectedPrinters" :options="printerOptions" label="Select Printers to Relay To" multiple
            emit-value map-options use-chips class="q-mb-md" />
          <q-btn type="submit" color="primary" label="Add Relay Group" />
        </q-form>
      </q-card-section>
      <q-separator />
      <q-card-section>
        <div class="text-subtitle1 q-mb-sm">Relay Groups</div>
        <q-list bordered separator>
          <q-item v-for="(group, idx) in relayGroups" :key="idx" clickable @click="selectedGroup = idx"
            :active="selectedGroup === idx">
            <q-item-section>
              <div>
                <span class="text-bold">Group {{ idx + 1 }}:</span>
                <span v-for="name in group.PrinterNames" :key="name" class="q-ml-sm">
                  {{ name }}
                </span>
              </div>
            </q-item-section>
            <q-item-section side>
              <q-btn flat icon="delete" color="negative" @click.stop="removeRelay(idx)" />
            </q-item-section>
            <q-item-section side v-if="selectedGroup === idx">
              <q-icon name="check_circle" color="primary" />
            </q-item-section>
          </q-item>
        </q-list>
      </q-card-section>
      <q-separator />
      <q-card-section>
        <div class="row q-gutter-md items-center">
          <q-btn @click="StartPrinter()" color="positive" :disable="serviceRunning || selectedGroup === null"
            label="Start Relay Service" />
          <q-btn @click="StopPrinter()" color="negative" :disable="!serviceRunning" label="Stop Relay Service" />
          <q-toggle v-model="serviceRunning" label="Service Running" color="purple-5" disable />
        </div>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { GetPrinters, StartPrinterServer, StopPrintServer, GetPrinterRunStatus, AddRelayGroup, GetRelayGroups, DeleteRelayGroup, SetPrinterRelayMode, SelectRelayGroup } from 'app/wailsjs/go/main/App'

const printerOptions = ref([])
const selectedPrinters = ref([])
const relayGroups = ref([])
const printers = ref([])
const selectedGroup = ref(null)
const serviceRunning = ref(false)

async function loadPrinters() {
  printers.value = await GetPrinters()
  printerOptions.value = printers.value.map(p => ({ label: p.printerName, value: p.printerID }))
  console.log('Printers:', printerOptions.value)
}

async function loadRelayGroups() {
  const groups = await GetRelayGroups()
  if (!groups || !Array.isArray(groups)) {
    relayGroups.value = []
    return
  }
  relayGroups.value = groups.map(g => {
    // Use printerOptions to get names for each printer ID
    const names = Array.isArray(g.printerIDs)
      ? g.printerIDs.map(pid => {
        const found = printerOptions.value.find(p => p.value === pid)
        return found ? found.label : pid
      })
      : []
    console.log('Relay Group:', g.groupID, 'Printer IDs:', g.printerIDs, 'Names:', names)
    return { GroupID: g.groupID, PrinterIDs: g.printerIDs || [], PrinterNames: names }
  })
}

async function addRelay() {
  if (selectedPrinters.value.length > 0) {
    await AddRelayGroup(selectedPrinters.value)
    selectedPrinters.value = []
    await loadRelayGroups()
  }
}
async function removeRelay(idx) {
  // Need to get the groupID from backend, so reload groups and use the index
  const groups = await GetRelayGroups()
  if (groups[idx]) {
    await DeleteRelayGroup(groups[idx].groupID)
    await loadRelayGroups()
  }
}

async function StartPrinter() {
  if (selectedGroup.value !== null) {
    serviceRunning.value = true
    await StartPrinterServer()
    await checkServiceStatus()
  }
}
async function StopPrinter() {
  serviceRunning.value = false
  await StopPrintServer()
  await checkServiceStatus()
}
async function checkServiceStatus() {
  serviceRunning.value = await GetPrinterRunStatus()
}


watch(selectedGroup, async (newVal) => {
  if (newVal !== null) {
    console.log('Selected Relay Group:', relayGroups.value[newVal])
    await SelectRelayGroup(relayGroups.value[newVal])
  }
})
onMounted(async () => {
  await SetPrinterRelayMode()
  await loadPrinters()
  await loadRelayGroups()
  await checkServiceStatus()
})
</script>
