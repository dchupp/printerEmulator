<template>
  <div class="label-card" :class="{ fresh }" @dblclick="emit('open')">
    <div class="label-paper" :style="paperStyle" @click="emit('open')">
      <img :src="label.url" :width="label.width" :height="label.height" loading="lazy" decoding="async" draggable="false" alt="Label" />
    </div>
    <div class="label-meta row items-center no-wrap">
      <div class="col ellipsis">
        <span class="text-weight-medium">{{ fmtTime(label.received) }}</span>
        <span class="text-grey-6"> · {{ labelInches(label) }}</span>
      </div>
      <q-icon v-if="label.warnings?.length" name="warning_amber" color="warning" size="16px" class="q-mr-xs">
        <q-tooltip max-width="420px">{{ label.warnings.length }} warning{{ label.warnings.length > 1 ? 's' : '' }} — open the label for details</q-tooltip>
      </q-icon>
      <q-btn flat round dense size="sm" icon="content_copy" @click.stop="copyLabelImage(label)"><q-tooltip>Copy image</q-tooltip></q-btn>
      <q-btn flat round dense size="sm" icon="more_vert" @click.stop>
        <q-menu anchor="bottom right" self="top right">
          <LabelActions :label="label" show-open @open="emit('open')" @remove="emit('remove')" />
        </q-menu>
      </q-btn>
    </div>
    <q-menu touch-position context-menu>
      <LabelActions :label="label" show-open @open="emit('open')" @remove="emit('remove')" />
    </q-menu>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import LabelActions from './LabelActions.vue'
import { fmtTime, labelInches } from '../state'
import { copyLabelImage } from '../labelActions'

const props = defineProps({ label: { type: Object, required: true }, zoom: { type: Number, default: 0.5 }, fresh: Boolean })
const emit = defineEmits(['open', 'remove'])

// Show labels at their physical proportions: zoom 1 = 96 CSS px per inch.
const paperStyle = computed(() => {
  const dpi = props.label.dpmm * 25.4
  const w = (props.label.width / dpi) * 96 * props.zoom * 2
  return { width: `${Math.max(90, Math.round(w))}px` }
})
</script>
