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
            <q-field class="q-mt-sm" flat dense>
              <template v-slot:control>
                <div class="self-center full-width no-outline" tabindex="0">{{ SavePath }}</div>
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

              <q-card v-for="print in Prints" class="q-ma-sm justify-center">
                <img class="justify-center" style="width: 35vw;" :src="`data:image/png;base64,${print}`" />
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
import { GetHeight, GetPrinterPort, GetWidth, GetPrinterRunStatus, SetPrintDirectory, UpdateHeight, UpdatePrinterPort, UpdatePrinterDPI, GetPrinterDPI, UpdateWidth, StartPrinterServer, StopPrintServer, UpdateSave } from 'app/wailsjs/go/main/App';
import { onMounted, ref, watch } from 'vue';
import { EventsOn } from "app/wailsjs/runtime/runtime";
const block = ref(false)
const PrinterOn = ref(false)
const PrinterSave = ref(false)
const PrintWidth = ref("4")
const PrintHeight = ref("8")
const PrinterPort = ref("9100")
const SavePath = ref("")
const PrinterDPI = ref({
  value: 8,
  desc: '8 dpmm (203 dpi)'
})
const Prints = ref([])

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

onMounted(async () => {
  await GetPrinterSetPoints()
  await CheckPrinterStatus()
})

EventsOn("NewPrint", function (data) {
  AddPrintToQueue(data)
});
EventsOn("Unblock", function () {
  block.value = false
});
const delay = ms => new Promise(res => setTimeout(res, ms));

async function CheckPrinterStatus() {
  while (true) {
    await GetPrinterStatus()
    await delay(5000);
    console.log(PrinterOn.value)
    console.log(block.value)
  }
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
function AddPrintToQueue(byteArray) {
  Prints.value = Prints.value.reverse()
  Prints.value.push(byteArray)
  Prints.value = Prints.value.reverse()
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
}
watch(PrintWidth, async () => {
  await UpdateWidth(parseInt(PrintWidth.value))
})
// watch(PrinterOn, async () => {
//   if (block.value == true) {
//     block.value == false
//   }
// })
watch(PrintHeight, async () => {
  await UpdateHeight(parseInt(PrintHeight.value))
})
watch(PrinterPort, async () => {
  await UpdatePrinterPort(parseInt(PrinterPort.value))
})
watch(PrinterDPI, async () => {
  await UpdatePrinterDPI(PrinterDPI.value)
})

watch(PrinterSave, async () => {
  await SetSaveFile()
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
</script>
