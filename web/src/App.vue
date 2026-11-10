<template>
  <v-app>
    <router-view />
  </v-app>
</template>

<style>
/* Material 3 buttons use sentence case, not Vuetify's shouting caps. Dialogs live outside .v-application,
   and the doubled class outranks Vuetify's rule whichever stylesheet loads last. */
.v-btn.v-btn {
  text-transform: none;
  letter-spacing: 0;
  font-weight: 600;
}
/* An "off" switch keeps a visible track, so it reads as a control on light cards. */
.v-switch .v-selection-control:not(.v-selection-control--dirty) .v-switch__track {
  background-color: rgba(var(--v-theme-on-surface), 0.32);
  opacity: 1;
}
/* On touch screens small buttons stay comfortably tappable. */
@media (pointer: coarse) {
  .v-btn--size-small:not(.v-btn--icon) {
    --v-btn-height: 36px;
  }
  .v-btn--icon.v-btn--size-small {
    --v-btn-size: 1.1rem;
    width: 40px;
    height: 40px;
  }
}
</style>
