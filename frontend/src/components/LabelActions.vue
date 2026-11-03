<template>
  <q-list dense style="min-width: 220px">
    <q-item v-if="showOpen" clickable v-close-popup @click="emit('open')">
      <q-item-section avatar class="act-icon"><q-icon name="open_in_full" size="20px" /></q-item-section>
      <q-item-section>Open</q-item-section>
    </q-item>
    <q-item clickable v-close-popup @click="copyLabelImage(label)">
      <q-item-section avatar class="act-icon"><q-icon name="content_copy" size="20px" /></q-item-section>
      <q-item-section>Copy image</q-item-section>
    </q-item>
    <q-item clickable v-close-popup @click="saveLabelAs(label)">
      <q-item-section avatar class="act-icon"><q-icon name="save_alt" size="20px" /></q-item-section>
      <q-item-section>Save as PNG…</q-item-section>
    </q-item>
    <q-item clickable v-close-popup @click="copyLabelZPL(label)">
      <q-item-section avatar class="act-icon"><q-icon name="data_object" size="20px" /></q-item-section>
      <q-item-section>Copy ZPL</q-item-section>
    </q-item>
    <q-item clickable v-close-popup :to="{ path: '/editor', query: { job: label.jobID } }">
      <q-item-section avatar class="act-icon"><q-icon name="edit_note" size="20px" /></q-item-section>
      <q-item-section>Open in ZPL editor</q-item-section>
    </q-item>
    <q-item clickable>
      <q-item-section avatar class="act-icon"><q-icon name="send" size="20px" /></q-item-section>
      <q-item-section>Send to printer</q-item-section>
      <q-item-section side><q-icon name="chevron_right" /></q-item-section>
      <q-menu anchor="top end" self="top start">
        <q-list dense style="min-width: 200px">
          <q-item v-for="p in state.printers" :key="p.printerID" clickable v-close-popup @click="sendLabelTo(label, p)">
            <q-item-section avatar class="act-icon"><q-icon :name="p.printerType === 'IPP' ? 'print' : 'qr_code_2'" size="20px" /></q-item-section>
            <q-item-section>
              <q-item-label>{{ p.printerName }}</q-item-label>
              <q-item-label caption>{{ p.printerType }} · {{ p.ipAddress }}</q-item-label>
            </q-item-section>
          </q-item>
          <q-item v-if="!state.printers.length" clickable v-close-popup to="/printers">
            <q-item-section avatar class="act-icon"><q-icon name="add" size="20px" /></q-item-section>
            <q-item-section>Add a network printer…</q-item-section>
          </q-item>
        </q-list>
      </q-menu>
    </q-item>
    <q-separator />
    <q-item clickable v-close-popup @click="emit('remove')">
      <q-item-section avatar class="act-icon"><q-icon name="delete_outline" color="negative" size="20px" /></q-item-section>
      <q-item-section class="text-negative">Remove</q-item-section>
    </q-item>
  </q-list>
</template>

<script setup>
import { state } from '../state'
import { copyLabelImage, copyLabelZPL, saveLabelAs, sendLabelTo } from '../labelActions'

defineProps({ label: { type: Object, required: true }, showOpen: Boolean })
const emit = defineEmits(['open', 'remove'])
</script>
