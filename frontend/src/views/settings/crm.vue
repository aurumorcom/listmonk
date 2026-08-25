<template>
  <div class="items">
    <div class="columns">
      <div class="column is-3">
        <b-field message="Enable Frappe / ERPNext CRM integration for automated deep research & subscriber synchronization.">
          <b-switch v-model="data.crm['enabled']" name="crm">
            Enable CRM Integration
          </b-switch>
        </b-field>
      </div>
      <div class="column is-9">
        <b-field label="CRM Base URL" label-position="on-border">
          <b-input
            v-model="data.crm['base_url']"
            name="crm.base_url"
            placeholder="https://crm.yoursite.com"
            :disabled="!data.crm['enabled']"
            :maxlength="300"
            type="url"
            pattern="https?://.*"
          />
        </b-field>

        <b-field label="API Key" label-position="on-border">
          <b-input
            v-model="data.crm['api_key']"
            name="crm.api_key"
            placeholder="Frappe CRM API Key"
            :disabled="!data.crm['enabled']"
            :maxlength="200"
          />
        </b-field>

        <b-field label="API Secret" label-position="on-border">
          <b-input
            v-model="data.crm['api_secret']"
            name="crm.api_secret"
            type="password"
            placeholder="Frappe CRM API Secret"
            :disabled="!data.crm['enabled']"
            :maxlength="200"
          />
        </b-field>
      </div>
    </div>
  </div>
</template>

<script>
import Vue from 'vue';

export default Vue.extend({
  props: {
    form: Object,
  },
  data() {
    return {
      data: this.form,
    };
  },
});
</script>
