<template>
  <q-page>
    <div class="row full-height">
      <div class="col-4">
        <q-card class="q-ma-sm">
          <q-card-section>
            <div class="text-h5">Printer Setup</div>
            <q-separator />
            <q-input color="purple-12" type="number" v-model="PrintWidth" label="Label Width">
              <template v-slot:prepend>
                <q-icon name="straighten" />
              </template>
            </q-input>
            <q-input color="purple-12" type="number" v-model="PrintHeight" label="Label Height">
              <template v-slot:prepend>
                <q-icon name="height" />
              </template>
            </q-input>

            <q-select v-model="PrinterDPI" :options="options" option-value="value" option-label="desc"
              label="Printer DPI">
              <template v-slot:prepend>
                <q-icon name="photo_size_select_small" />
              </template>
            </q-select>

            <q-select v-model="Rotation" :options="rotationOptions" label="Print Rotation">
              <template v-slot:prepend>
                <q-icon name="rotate_90_degrees_cw" />
              </template>
            </q-select>
            <q-input v-if="PrinterOn == false" color="purple-12" type="number" v-model="PrinterPort"
              label="Printer Port">
              <template v-slot:prepend>
                <q-icon name="u_turn_right" />
              </template>
            </q-input>
            <q-input v-else disable color="purple-12" type="number" v-model="PrinterPort" label="Printer Port">
              <template v-slot:prepend>
                <q-icon name="u_turn_right" />
              </template>
            </q-input>
            <q-btn @click="StartPrinter()" class="q-ma-sm" size="sm" color="positive"
              v-if="PrinterOn == false && block == false">Start
              Printer</q-btn>
            <q-btn flat class="q-ma-sm" size="sm" color="positive" v-else disable>Start Printer</q-btn>

            <q-btn @click="StopPrinter()" size="sm" class="q-ma-sm" color="negative"
              v-if="PrinterOn == true && block == false">Stop
              Printer</q-btn>
            <q-btn flat size="sm" class="q-ma-sm" color="negative" v-else disable>Stop Printer</q-btn>

            <q-toggle size="lg" v-if="PrinterOn == true" v-model="PrinterSave" label="Save Prints" color="purple-5" />
            <q-btn @click="GetSaveFileLocation()" color="" flat class="bg-purple-1 full-width" dense>Select Save
              Directory</q-btn>
            <q-separator class="q-my-sm" />
            <q-toggle size="lg" v-model="AutoStartEnabled" label="Start at Windows Login" color="purple-5" />
            <q-toggle size="lg" v-model="AutoStartServerEnabled" label="Auto-start Printer Server" color="purple-5" />
            <q-field class="q-mt-sm" flat dense>
              <template v-slot:control>
                <div class="row items-center full-width no-outline" tabindex="0">
                  <div class="col-grow">{{ SavePath }}</div>
                  <q-btn v-if="SavePath" size="xs" flat round icon="close" @click="ClearPrintPath" />
                </div>
              </template>
            </q-field>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-8">
        <q-card class="q-ml-sm bg-grey-2" style="height: 88vh; max-width: 95%;">

          <div style="height: 86vh; max-width: 98%;overflow: auto">
            <div class="row justify-center q-gutter-sm">
              <!-- eslint-disable -->

              <q-card v-for="(print, index) in Prints" :key="index" class="q-ma-sm justify-center">
                <img class="justify-center" style="width: fit-content; cursor: pointer;" :src="`data:image/png;base64,${print}`" />
                <q-menu touch-position context-menu>
                  <q-list dense style="min-width: 150px">
                    <q-item clickable v-close-popup @click="copyToClipboard(print)">
                      <q-item-section avatar>
                        <q-icon name="content_copy" size="sm" />
                      </q-item-section>
                      <q-item-section>Copy to Clipboard</q-item-section>
                    </q-item>
                    <q-item clickable v-close-popup @click="removePrint(index)">
                      <q-item-section avatar>
                        <q-icon name="delete" size="sm" color="negative" />
                      </q-item-section>
                      <q-item-section>Remove</q-item-section>
                    </q-item>
                  </q-list>
                </q-menu>
              </q-card>
              <!-- eslint-enable -->

            </div>
          </div>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup>
import { GetHeight, GetPrinterPort, GetWidth, GetPrinterRunStatus, SetPrintDirectory, UpdateHeight, UpdatePrinterPort, UpdatePrinterDPI, GetPrinterDPI, UpdateWidth, StartPrinterServer, StopPrintServer, UpdateSave, GetPrinterRotation, SetPrinterRotation, GetPrintDirectory, ClearPrintDirectory, SetPrinterEmulatorMode, GetPrinters, GetAutoStart, SetAutoStart, GetAutoStartServer, SetAutoStartServer } from 'app/wailsjs/go/main/App';
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EventsOn, EventsOff } from "app/wailsjs/runtime/runtime";

const block = ref(false)
let statusPollInterval = null
let debounceTimers = {}
const PrinterOn = ref(false)
const PrinterSave = ref(false)
const AutoStartEnabled = ref(false)
const AutoStartServerEnabled = ref(false)
const PrintWidth = ref("4")
const PrintHeight = ref("8")
const PrinterPort = ref("9100")
const SavePath = ref("")
const PrinterDPI = ref({
  value: 8,
  desc: '8 dpmm (203 dpi)'
})
const Prints = ref([])

const Rotation = ref(0)

const rotationOptions = [0, 90, 180, 270]

// onMounted
const options = [
  {
    value: 6,
    desc: '6 dpmm (152 dpi)'
  },
  {
    value: 8,
    desc: '8 dpmm (203 dpi)'
  },
  {
    value: 12,
    desc: '12 dpmm (300 dpi)'
  },
  {
    value: 24,
    desc: '24 dpmm (600 dpi)'
  }
]

const selectedPrinter = ref(null)
const printerOptions = ref([])
const selectedPrinterType = ref('')
const printDestination = ref('printer')
const printDestinationOptions = [
  { label: 'Printer', value: 'printer' },
  { label: 'Screen', value: 'screen' }
]
const ippDisplayOption = ref('show')
const ippDisplayOptions = [
  { label: 'Display Print', value: 'show' },
  { label: 'Do Not Display Print', value: 'hide' }
]

async function loadPrinterOptions() {
  const printers = await GetPrinters()
  printerOptions.value = printers.map(p => ({ label: p.printerName, value: p.printerID, type: p.printerType }))
}

onMounted(async () => {
  await SetPrinterEmulatorMode()
  await GetPrinterSetPoints()
  await loadAutoStartStatus()
  await GetRotation()
  await loadPrinterOptions()
  // Start polling with interval instead of infinite loop
  statusPollInterval = setInterval(async () => {
    await GetPrinterStatus()
  }, 5000)
})

onUnmounted(() => {
  // Cleanup polling interval
  if (statusPollInterval) {
    clearInterval(statusPollInterval)
    statusPollInterval = null
  }
  // Cleanup debounce timers
  Object.values(debounceTimers).forEach(timer => clearTimeout(timer))
  debounceTimers = {}
  // Cleanup event listeners
  EventsOff("NewPrint")
  EventsOff("Unblock")
})

async function loadAutoStartStatus() {
  AutoStartEnabled.value = await GetAutoStart()
  AutoStartServerEnabled.value = await GetAutoStartServer()
}

EventsOn("NewPrint", function (data) {
  AddPrintToQueue(data)
});
EventsOn("Unblock", function () {
  block.value = false
});

// Debounce helper function
function debounce(key, fn, delay = 500) {
  if (debounceTimers[key]) {
    clearTimeout(debounceTimers[key])
  }
  debounceTimers[key] = setTimeout(fn, delay)
}
async function SetSaveFile() {
  await UpdateSave(PrinterSave.value)
}
async function GetSaveFileLocation() {
  await SetPrintDirectory().then((result) => {
    if (result != null) {
      SavePath.value = result
      return
    }
  });
}
async function GetRotation() {
  await GetPrinterRotation().then((result) => {
    if (result != null) {
      Rotation.value = result
      return
    }
  });
}
function AddPrintToQueue(byteArray) {
  // Use unshift to prepend instead of double-reverse (O(n) vs O(2n))
  Prints.value.unshift(byteArray)
}
function ClearPrints() {
  Prints.value = []
}
async function GetPrinterSetPoints() {
  await GetHeight().then((result) => {
    if (result != null) {
      PrintHeight.value = result
      return
    }
  });
  await GetWidth().then((result) => {
    console.log(result)
    if (result != null) {
      PrintWidth.value = result
      return
    }

  });
  await GetPrinterPort().then((result) => {
    if (result != null) {
      PrinterPort.value = result
      return
    }
  });
  await GetPrinterRunStatus().then((result) => {
    if (result != null) {
      PrinterOn.value = result
      return
    }
  });
  await GetPrinterDPI().then((result) => {
    if (result != null) {
      PrinterDPI.value = result
      return
    }
  });
  await GetPrinterRotation().then((result) => {
    if (result != null) {
      Rotation.value = result
      return
    }
  });
  await GetPrintDirectory().then((result) => {
    if (result != null) {
      SavePath.value = result
      return
    }
  });
}
// Debounced watchers to prevent excessive API calls
watch(PrintWidth, () => {
  debounce('width', async () => {
    await UpdateWidth(parseInt(PrintWidth.value))
  })
})
watch(Rotation, () => {
  debounce('rotation', async () => {
    await SetPrinterRotation(Rotation.value)
  })
})
watch(PrintHeight, () => {
  debounce('height', async () => {
    await UpdateHeight(parseInt(PrintHeight.value))
  })
})
watch(PrinterPort, () => {
  debounce('port', async () => {
    await UpdatePrinterPort(parseInt(PrinterPort.value))
  })
})
watch(PrinterDPI, () => {
  debounce('dpi', async () => {
    await UpdatePrinterDPI(PrinterDPI.value)
  })
})

watch(PrinterSave, async () => {
  await SetSaveFile()
})

watch(AutoStartEnabled, async () => {
  await SetAutoStart(AutoStartEnabled.value)
})

watch(AutoStartServerEnabled, async () => {
  await SetAutoStartServer(AutoStartServerEnabled.value)
})
async function GetPrinterStatus() {
  await GetPrinterRunStatus().then((result) => {
    if (result != null) {
      PrinterOn.value = result
      return
    }
  })
}
async function StartPrinter() {
  try {
    block.value = true
    await StartPrinterServer()
    await GetPrinterStatus()
  } catch (error) {
    console.log(error)
  } finally {
  }

}
async function StopPrinter() {
  try {
    block.value = true
    await StopPrintServer()
    await GetPrinterStatus()

  } catch (error) {
    console.log(error)
  } finally {
  }

}
async function ClearPrintPath() {
  SavePath.value = ""
  ClearPrintDirectory()
  console.log("Cleared")
}
watch(selectedPrinter, (val) => {
  const found = printerOptions.value.find(p => p.value === val)
  selectedPrinterType.value = found ? found.type : ''
})

// Copy base64 PNG image to clipboard
async function copyToClipboard(base64Data) {
  try {
    // Convert base64 to blob
    const response = await fetch(`data:image/png;base64,${base64Data}`)
    const blob = await response.blob()

    // Use Clipboard API to write the image
    await navigator.clipboard.write([
      new ClipboardItem({
        [blob.type]: blob
      })
    ])
    console.log('Image copied to clipboard')
  } catch (err) {
    console.error('Failed to copy image to clipboard:', err)
    // Fallback: try to copy as text (base64)
    try {
      await navigator.clipboard.writeText(base64Data)
      console.log('Base64 data copied to clipboard as text')
    } catch (fallbackErr) {
      console.error('Fallback copy also failed:', fallbackErr)
    }
  }
}

// Remove a print from the queue
function removePrint(index) {
  Prints.value.splice(index, 1)
}
</script>
