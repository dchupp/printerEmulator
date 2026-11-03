<template>
  <q-dialog :model-value="modelValue" maximized transition-show="fade" transition-hide="fade" @update:model-value="v => emit('update:modelValue', v)" @keydown="onKey">
    <div v-if="label" class="viewer row no-wrap">
      <!-- Image -->
      <div class="col viewer-stage column no-wrap">
        <div class="row items-center q-px-md q-py-sm viewer-toolbar">
          <q-btn flat round dense icon="chevron_left" :disable="index >= labels.length - 1" @click="go(1)"><q-tooltip>Older (←)</q-tooltip></q-btn>
          <div class="q-mx-sm text-caption">{{ labels.length - index }} / {{ labels.length }}</div>
          <q-btn flat round dense icon="chevron_right" :disable="index <= 0" @click="go(-1)"><q-tooltip>Newer (→)</q-tooltip></q-btn>
          <q-space />
          <q-btn-toggle v-model="fit" dense unelevated no-caps toggle-color="primary" :options="[{ label: 'Fit', value: true }, { label: 'Actual dots', value: false }]" />
        </div>
        <div class="col viewer-canvas" :class="{ fit }">
          <div class="label-paper big">
            <img :src="label.url" :width="label.width" :height="label.height" draggable="false" alt="Label" />
          </div>
        </div>
      </div>

      <!-- Details -->
      <div class="viewer-side column no-wrap">
        <div class="row items-center q-pa-md">
          <div class="text-h6">Label #{{ label.id }}</div>
          <q-space />
          <q-btn flat round dense icon="close" v-close-popup />
        </div>
        <q-tabs v-model="tab" dense align="left" no-caps active-color="primary" indicator-color="primary" class="q-px-sm">
          <q-tab name="details" label="Details" />
          <q-tab name="zpl" label="ZPL" />
          <q-tab v-if="label.warnings?.length" name="warnings">
            <div class="row items-center no-wrap">Warnings <q-badge color="warning" text-color="dark" class="q-ml-xs">{{ label.warnings.length }}</q-badge></div>
          </q-tab>
        </q-tabs>
        <q-separator />
        <div class="col scroll">
          <q-tab-panels v-model="tab" animated class="bg-transparent">
            <q-tab-panel name="details" class="q-pa-none">
              <q-list dense class="q-py-sm">
                <q-item><q-item-section>Received</q-item-section><q-item-section side>{{ fmtDateTime(label.received) }}</q-item-section></q-item>
                <q-item><q-item-section>From</q-item-section><q-item-section side>{{ label.source }}</q-item-section></q-item>
                <q-item><q-item-section>Size</q-item-section><q-item-section side>{{ labelInches(label) }}</q-item-section></q-item>
                <q-item><q-item-section>Dots</q-item-section><q-item-section side>{{ label.width }} × {{ label.height }}</q-item-section></q-item>
                <q-item><q-item-section>Density</q-item-section><q-item-section side>{{ label.dpmm }} dpmm ({{ Math.round(label.dpmm * 25.4) }} dpi)</q-item-section></q-item>
                <q-item><q-item-section>PNG</q-item-section><q-item-section side>{{ fmtBytes(label.bytes) }}</q-item-section></q-item>
                <q-item><q-item-section>Job</q-item-section><q-item-section side>#{{ label.jobID }}, label {{ label.index + 1 }}</q-item-section></q-item>
              </q-list>
              <q-separator />
              <LabelActions :label="label" class="q-py-sm" @remove="removeCurrent" />
            </q-tab-panel>
            <q-tab-panel name="zpl" class="q-pa-sm">
              <div v-if="zplError" class="text-caption text-grey-6 q-pa-sm">{{ zplError }}</div>
              <pre v-else class="zpl-source">{{ zpl }}</pre>
            </q-tab-panel>
            <q-tab-panel name="warnings">
              <div v-for="(w, i) in label.warnings" :key="i" class="warning-line">{{ w }}</div>
            </q-tab-panel>
          </q-tab-panels>
        </div>
      </div>
    </div>
  </q-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import LabelActions from './LabelActions.vue'
import { api, state, fmtDateTime, fmtBytes, labelInches } from '../state'

const props = defineProps({ modelValue: Boolean, index: { type: Number, default: 0 }, labels: { type: Array, required: true } })
const emit = defineEmits(['update:modelValue', 'update:index'])

const label = computed(() => props.labels[props.index])
const fit = ref(true)
const tab = ref('details')
const zpl = ref('')
const zplError = ref('')

watch([() => label.value?.jobID, tab], async ([jobID, t]) => {
  if (t !== 'zpl' || jobID == null) return
  zplError.value = ''
  try {
    zpl.value = await api.JobZPL(jobID)
  } catch (e) {
    zplError.value = e.message || String(e)
  }
})
watch(() => label.value?.id, () => { if (tab.value === 'warnings' && !label.value?.warnings?.length) tab.value = 'details' })
watch(() => props.labels.length, n => { if (props.modelValue && n === 0) emit('update:modelValue', false) })

function go (d) {
  const i = props.index + d
  if (i >= 0 && i < props.labels.length) emit('update:index', i)
}
function onKey (e) {
  if (e.key === 'ArrowLeft') go(1)
  else if (e.key === 'ArrowRight') go(-1)
}
async function removeCurrent () {
  const l = label.value
  await api.RemoveLabel(l.id)
  const i = state.labels.findIndex(x => x.id === l.id)
  if (i >= 0) state.labels.splice(i, 1)
  if (props.index >= state.labels.length) emit('update:index', Math.max(0, state.labels.length - 1))
}
</script>
