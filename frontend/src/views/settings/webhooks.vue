<template>
  <div>
    <div class="items webhooks-list">
      <div class="block box" v-for="(item, n) in data.webhooks" :key="n">
        <div class="columns">
          <div class="column is-2">
            <b-field>
              <b-switch v-model="item.enabled" name="enabled" :native-value="true" data-cy="btn-enable-webhook">
                {{ $t('globals.buttons.enabled') }}
              </b-switch>
            </b-field>
            <b-field v-if="data.webhooks && data.webhooks.length > 1">
              <a @click.prevent="$utils.confirm(null, () => removeWebhook(n))" href="#" data-cy="btn-delete-webhook">
                <b-icon icon="trash-can-outline" />
                {{ $t('globals.buttons.delete') }}
              </a>
            </b-field>
          </div><!-- first column -->

          <div class="column" :class="{ disabled: !item.enabled }">
            <div class="columns">
              <div class="column is-4">
                <b-field label="Name" label-position="on-border" message="Name identifier for this outbound endpoint">
                  <b-input v-model="item.name" name="name" placeholder="n8n Production Sync" :maxlength="100" />
                </b-field>
              </div>
              <div class="column is-8">
                <b-field label="Request URL" label-position="on-border" message="HTTP(S) target endpoint receiving JSON event snapshots">
                  <b-input v-model="item.url" name="url" type="url" placeholder="https://n8n.example.com/webhook/listmonk" :maxlength="500" />
                </b-field>
              </div>
            </div><!-- name & url -->

            <div class="columns">
              <div class="column is-6">
                <b-field label="Webhook Secret" label-position="on-border" message="Secret key used for Listmonk-Signature HTTP header verification">
                  <b-input v-model="item.secret" name="secret" type="password" password-reveal placeholder="whsec_..." :maxlength="200" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field label="Custom HTTP Headers" label-position="on-border" message="Optional custom headers as JSON array">
                  <b-input v-model="item.strHeaders" name="headers" placeholder="[{&quot;X-Custom&quot;: &quot;value&quot;}]" />
                </b-field>
              </div>
            </div><!-- secret & headers -->

            <div class="spaced-links is-size-7 mb-3">
              <a href="#" @click.prevent="() => fillPreset(n, 'n8n')">n8n Workflow</a>
              <a href="#" @click.prevent="() => fillPreset(n, 'zapier')">Zapier Webhook</a>
              <a href="#" @click.prevent="() => fillPreset(n, 'make')">Make (Integromat)</a>
              <a href="#" @click.prevent="() => fillPreset(n, 'hubspot')">HubSpot Sync</a>
              <a href="#" @click.prevent="() => fillPreset(n, 'custom')">Custom Endpoint</a>
            </div>

            <!-- Event Subscriptions Box -->
            <div class="box mt-4">
              <h5 class="title is-6 mb-3">Event Subscriptions</h5>
              <p class="is-size-7 has-text-grey mb-3">Select trigger events that transmit full state snapshots to this webhook target:</p>

              <div class="columns is-multiline">
                <!-- Subscribers -->
                <div class="column is-6">
                  <div class="label is-size-7 mb-2"><strong>Subscribers</strong></div>
                  <div class="field" v-for="evt in subscriberEvents" :key="evt.value">
                    <b-checkbox v-model="item.events" :native-value="evt.value">
                      {{ evt.label }}
                    </b-checkbox>
                  </div>
                </div>

                <!-- Contacts -->
                <div class="column is-6">
                  <div class="label is-size-7 mb-2"><strong>Contacts</strong></div>
                  <div class="field" v-for="evt in contactEvents" :key="evt.value">
                    <b-checkbox v-model="item.events" :native-value="evt.value">
                      {{ evt.label }}
                    </b-checkbox>
                  </div>
                </div>

                <!-- Sequences -->
                <div class="column is-6">
                  <div class="label is-size-7 mb-2"><strong>Sequences</strong></div>
                  <div class="field" v-for="evt in sequenceEvents" :key="evt.value">
                    <b-checkbox v-model="item.events" :native-value="evt.value">
                      {{ evt.label }}
                    </b-checkbox>
                  </div>
                </div>

                <!-- Campaigns -->
                <div class="column is-6">
                  <div class="label is-size-7 mb-2"><strong>Campaigns</strong></div>
                  <div class="field" v-for="evt in campaignEvents" :key="evt.value">
                    <b-checkbox v-model="item.events" :native-value="evt.value">
                      {{ evt.label }}
                    </b-checkbox>
                  </div>
                </div>
              </div>
            </div><!-- Event Subscriptions Box -->

            <hr />

            <!-- Test Connection Form -->
            <form @submit.prevent="() => doWebhookTest(item)">
              <div class="columns">
                <template v-if="webhookTestItem === n">
                  <div class="column is-5">
                    <strong>Select Test Event Type</strong>
                    <b-select v-model="testEventType" expanded class="mt-1">
                      <option value="subscriber.created">subscriber.created</option>
                      <option value="contact.created">contact.created</option>
                      <option value="sequence.step_executed">sequence.step_executed</option>
                      <option value="campaign.status_changed">campaign.status_changed</option>
                    </b-select>
                  </div>
                  <div class="column is-4">
                    <b-field label="Request URL" label-position="on-border">
                      <b-input type="url" required v-model="item.url" placeholder="https://..." :custom-class="`test-url-${n}`" />
                    </b-field>
                  </div>
                </template>
                <div class="column has-text-right">
                  <b-button v-if="webhookTestItem === n" class="is-primary" @click.prevent="() => doWebhookTest(item)">
                    Send Test Event
                  </b-button>
                  <a href="#" v-else class="is-primary" @click.prevent="showTestForm(n)">
                    <b-icon icon="rocket-launch-outline" /> Test Endpoint Connection
                  </a>
                </div>
              </div>
              <div v-if="errMsg && webhookTestItem === n">
                <b-field class="mt-4" type="is-danger">
                  <b-input v-model="errMsg" type="textarea" custom-class="has-text-danger is-size-6" readonly />
                </b-field>
              </div>
            </form><!-- webhook test -->
          </div>
        </div><!-- second container column -->
      </div><!-- block -->
    </div><!-- webhooks-list -->

    <b-button @click="addWebhook" icon-left="plus" type="is-primary">
      {{ $t('globals.buttons.addNew') }}
    </b-button>
  </div>
</template>

<script>
import Vue from 'vue';

const presetTemplates = {
  n8n: {
    name: 'n8n Production Sync',
    url: 'https://n8n.example.com/webhook/listmonk',
    events: ['contact.created', 'contact.updated', 'sequence.step_executed'],
  },
  zapier: {
    name: 'Zapier Catch Webhook',
    url: 'https://hooks.zapier.com/hooks/catch/...',
    events: ['contact.created', 'contact.unsubscribed'],
  },
  make: {
    name: 'Make Scenario Hook',
    url: 'https://hook.eu1.make.com/...',
    events: ['contact.created', 'campaign.status_changed'],
  },
  hubspot: {
    name: 'HubSpot Contact Sync',
    url: 'https://api.hubapi.com/...',
    events: ['contact.created', 'contact.updated', 'contact.deleted'],
  },
  custom: {
    name: 'Custom Outbound Receiver',
    url: 'https://api.yourdomain.com/webhooks/listmonk',
    events: [
      'contact.created',
      'contact.updated',
      'contact.unsubscribed',
      'contact.bounced',
      'sequence.step_executed',
      'campaign.status_changed',
    ],
  },
};

export default Vue.extend({
  props: {
    form: {
      type: Object,
      default: () => ({}),
    },
  },

  data() {
    return {
      data: this.form,
      webhookTestItem: null,
      testEventType: 'subscriber.created',
      errMsg: '',
      subscriberEvents: [
        { value: 'subscriber.created', label: 'subscriber.created' },
        { value: 'subscriber.updated', label: 'subscriber.updated' },
        { value: 'subscriber.unsubscribed', label: 'subscriber.unsubscribed' },
        { value: 'subscriber.deleted', label: 'subscriber.deleted' },
        { value: 'subscriber.bounced', label: 'subscriber.bounced' },
      ],
      contactEvents: [
        { value: 'contact.created', label: 'contact.created' },
        { value: 'contact.updated', label: 'contact.updated' },
        { value: 'contact.unsubscribed', label: 'contact.unsubscribed' },
        { value: 'contact.deleted', label: 'contact.deleted' },
        { value: 'contact.bounced', label: 'contact.bounced' },
      ],
      sequenceEvents: [
        { value: 'sequence.created', label: 'sequence.created' },
        { value: 'sequence.contact_enrolled', label: 'sequence.contact_enrolled' },
        { value: 'sequence.step_executed', label: 'sequence.step_executed' },
        { value: 'sequence.contact_replied', label: 'sequence.contact_replied' },
        { value: 'sequence.contact_completed', label: 'sequence.contact_completed' },
      ],
      campaignEvents: [
        { value: 'campaign.status_changed', label: 'campaign.status_changed' },
        { value: 'campaign.sent', label: 'campaign.sent' },
      ],
    };
  },

  mounted() {
    if (!this.data.webhooks || !Array.isArray(this.data.webhooks) || this.data.webhooks.length === 0) {
      this.$set(this.data, 'webhooks', [
        {
          name: '',
          enabled: true,
          url: '',
          secret: '',
          events: ['subscriber.created', 'subscriber.updated', 'contact.created', 'contact.updated'],
          strHeaders: '',
        },
      ]);
    }
  },

  methods: {
    addWebhook() {
      if (!this.data.webhooks) {
        this.$set(this.data, 'webhooks', []);
      }
      this.data.webhooks.push({
        name: '',
        enabled: true,
        url: '',
        secret: '',
        events: ['contact.created', 'contact.updated'],
        strHeaders: '',
      });

      this.$nextTick(() => {
        const items = document.querySelectorAll('.webhooks-list input[name="url"]');
        if (items.length > 0) {
          items[items.length - 1].focus();
        }
      });
    },

    removeWebhook(i) {
      this.data.webhooks.splice(i, 1);
    },

    showTestForm(n) {
      this.webhookTestItem = n;
      this.errMsg = '';
      this.$nextTick(() => {
        const i = document.querySelector(`.test-url-${n}`);
        if (i) i.focus();
      });
    },

    doWebhookTest(item) {
      if (!item.url) {
        this.$utils.toast('Please enter a target URL before testing.', 'is-danger');
        return;
      }

      this.errMsg = '';
      this.$api.testWebhook({
        url: item.url,
        secret: item.secret,
        event_type: this.testEventType,
      }).then(() => {
        this.$utils.toast('Test event payload dispatched successfully.');
      }).catch((err) => {
        if (err.response?.data?.message) {
          this.errMsg = err.response.data.message;
        } else {
          this.errMsg = err.message || 'Error executing webhook test';
        }
      });
    },

    fillPreset(n, key) {
      if (!presetTemplates[key]) return;
      const tpl = presetTemplates[key];
      this.data.webhooks.splice(n, 1, {
        ...this.data.webhooks[n],
        name: tpl.name,
        url: tpl.url,
        events: [...tpl.events],
      });
    },
  },
});
</script>
