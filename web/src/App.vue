<template>
  <v-app>
    <router-view />
  </v-app>
</template>

<style>
/* Material 3 buttons use sentence case, not Vuetify's shouting caps. */
.v-application .v-btn {
  text-transform: none;
  letter-spacing: 0;
  font-weight: 600;
}
</style>
