<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar class="bg-deep-purple-12">
        <q-toolbar-title>
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


    <q-page-container>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { BrowserOpenURL } from 'app/wailsjs/runtime/runtime';
function GoToLinkedin() {
  BrowserOpenURL('https://www.linkedin.com/in/david-chupp/')
}
function GoToGitHub() {
  BrowserOpenURL('https://github.com/dchupp/printerEmulator')
}
</script>
