<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar class="bg-deep-purple-12">
        <q-toolbar-title>
          <q-btn flat dense round icon="menu" @click="drawer = !drawer" aria-label="Menu" class="q-mr-md" />
          Printer Emulator
        </q-toolbar-title>

        <q-btn flat dense color="primary" @click="GoToGitHub" class=" text-white q-mr-s"><q-icon class="text-h4"
            name="code" />
          @github/Dchupp</q-btn>
        <q-btn dense flat color="primary" class="text-white" @click="GoToLinkedin"><q-icon class="text-h4"
            name="perm_contact_calendar" />
          @linkedin/Dchupp</q-btn>

        <div>
          <q-icon name="electric_bolt" /> v2.2.2
        </div>
      </q-toolbar>
    </q-header>


    <q-drawer v-model="drawer" show-if-above side="left" bordered>
      <q-list>
        <q-item clickable v-ripple to="/" @click="drawer = false">
          <q-item-section avatar><q-icon name="home" /></q-item-section>
          <q-item-section>Home</q-item-section>
        </q-item>
        <q-item clickable v-ripple to="/printers" @click="drawer = false">
          <q-item-section avatar><q-icon name="print" /></q-item-section>
          <q-item-section>Printers</q-item-section>
        </q-item>
        <q-item clickable v-ripple to="/relay" @click="drawer = false">
          <q-item-section avatar><q-icon name="sync_alt" /></q-item-section>
          <q-item-section>Printer Relay Menu</q-item-section>
        </q-item>
        <q-item clickable v-ripple to="/zpl2net" @click="drawer = false">
          <q-item-section avatar><q-icon name="lan" /></q-item-section>
          <q-item-section>ZPL to Network Printer</q-item-section>
        </q-item>
      </q-list>
    </q-drawer>

    <q-page-container>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { BrowserOpenURL } from 'app/wailsjs/runtime/runtime';
import { ref } from 'vue'
function GoToLinkedin() {
  BrowserOpenURL('https://www.linkedin.com/in/david-chupp/')
}
function GoToGitHub() {
  BrowserOpenURL('https://github.com/dchupp/printerEmulator')
}
const drawer = ref(false)
</script>
