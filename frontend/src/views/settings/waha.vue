<template>
  <div>
    <div class="items waha-accounts">
      <div class="block box" v-for="(item, n) in data.waha" :key="n">
        <div class="columns">
          <!-- Left Column (2) with Enabled Switch & Delete Link matching SMTP.vue -->
          <div class="column is-2">
            <b-field>
              <b-switch v-model="item.enabled" name="enabled" :native-value="true" data-cy="btn-enable-waha">
                {{ $t('globals.buttons.enabled') }}
              </b-switch>
            </b-field>
            <b-field v-if="data.waha.length > 1">
              <a @click.prevent="$utils.confirm(null, () => removeMessenger(n))" href="#" data-cy="btn-delete-waha">
                <b-icon icon="trash-can-outline" />
                {{ $t('globals.buttons.delete') }}
              </a>
            </b-field>
          </div>

          <!-- Main Container Column matching SMTP.vue -->
          <div class="column" :class="{ disabled: !item.enabled }">
            <div class="columns">
              <div class="column is-6">
                <b-field :label="$t('globals.fields.name')" label-position="on-border" message="Unique identifier for this WhatsApp messenger">
                  <b-input v-model="item.name" name="name" placeholder="whatsapp" :maxlength="100" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field label="Root URL" label-position="on-border" message="HTTP endpoint of the WAHA container">
                  <b-input v-model="item.root_url" name="root_url" placeholder="http://waha:3000" :maxlength="200" expanded type="url" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-4">
                <b-field label="API Key" label-position="on-border" message="X-Api-Key header authentication">
                  <b-input v-model="item.api_key" name="api_key" type="password" placeholder="Secret API key" :maxlength="200" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Session ID" label-position="on-border" message="WAHA WhatsApp session name (default: 'default')">
                  <b-input v-model="item.session" name="session" placeholder="default" :maxlength="100" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Phone Attribute Key" label-position="on-border" message="JSON key in subscriber attribs for phone number">
                  <b-input v-model="item.phone_attribute" name="phone_attribute" placeholder="phone" :maxlength="100" />
                </b-field>
              </div>
            </div>

            <hr />

            <div class="columns">
              <div class="column is-4">
                <b-field label="Max Connections" label-position="on-border">
                  <b-numberinput v-model="item.max_conns" name="max_conns" type="is-light" controls-position="compact" placeholder="10" min="1" max="1000" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Max Retries" label-position="on-border">
                  <b-numberinput v-model="item.max_msg_retries" name="max_msg_retries" type="is-light" controls-position="compact" placeholder="2" min="1" max="100" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Timeout" label-position="on-border">
                  <b-input v-model="item.timeout" name="timeout" placeholder="10s" :maxlength="10" />
                </b-field>
              </div>
            </div>

            <!-- Standardized Sending Limits Section -->
            <div class="columns">
              <div class="column is-6">
                <b-field label="Messages sent per day" label-position="on-border" message="Daily max sending quota for this WhatsApp session (0 = unlimited)">
                  <b-numberinput v-model="item.messages_per_day" name="messages_per_day" type="is-light" controls-position="compact" placeholder="0" min="0" max="100000" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field label="Messages sent per hour" label-position="on-border" message="Hourly max sending quota for this WhatsApp session (0 = unlimited)">
                  <b-numberinput v-model="item.messages_per_hour" name="messages_per_hour" type="is-light" controls-position="compact" placeholder="0" min="0" max="10000" />
                </b-field>
              </div>
            </div>

            <hr />

            <!-- Human Typing Settings (Simple inline section without box container or headline) -->
            <div class="columns">
              <div class="column is-4">
                <b-field label="Simulation Mode" label-position="on-border" message="Default: Full Human Markov Simulation">
                  <b-select v-model="item.typing_mode" placeholder="human" expanded>
                    <option value="human">Human Markov Simulation (Default)</option>
                    <option value="simple">Simple Delay</option>
                    <option value="off">Disabled</option>
                  </b-select>
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Target WPM" label-position="on-border" message="Base typing speed (default: 60 WPM)">
                  <b-numberinput v-model="item.target_wpm" name="target_wpm" type="is-light" controls-position="compact" placeholder="60" min="10" max="200" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="WPM Standard Deviation" label-position="on-border" message="Speed variance per session (default: 10)">
                  <b-numberinput v-model="item.wpm_std" name="wpm_std" type="is-light" controls-position="compact" placeholder="10" min="0" max="50" step="0.5" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-6">
                <b-field label="Keyboard Layout" label-position="on-border" message="Physical layout for neighbor error generation">
                  <b-select v-model="item.keyboard_layout" placeholder="qwerty" expanded>
                    <option value="qwerty">QWERTY (Default)</option>
                    <option value="azerty">AZERTY</option>
                  </b-select>
                </b-field>
              </div>
              <div class="column is-6">
                <b-field label="Max Typing Pause Cap (Sec)" label-position="on-border" message="Ceiling limit to prevent campaign delay (default: 30s)">
                  <b-numberinput v-model="item.max_typing_delay_sec" name="max_typing_delay_sec" type="is-light" controls-position="compact" placeholder="30" min="1" max="300" />
                </b-field>
              </div>
            </div>

            <hr />

            <!-- Signature Section -->
            <div class="columns">
              <div class="column is-12">
                <b-field label="Signature" label-position="on-border" message="Default signature appended to WhatsApp messages sent from this session">
                  <b-input v-model="item.signature" name="signature" type="textarea" rows="2" placeholder="Best regards,\nSupport Team" />
                </b-field>
              </div>
            </div>

            <!-- User Section (Below Signature) -->
            <div class="columns">
              <div class="column is-12">
                <b-field label="User" label-position="on-border" message="Team member assigned to this WhatsApp account">
                  <b-select v-model="item.user_id" expanded>
                    <option :value="null">&mdash; None &mdash;</option>
                    <option v-for="u in users" :value="u.id" :key="u.id">
                      {{ u.name || u.username }} ({{ u.email || u.username }})
                    </option>
                  </b-select>
                </b-field>
              </div>
            </div>

            <hr />

            <div class="columns">
              <div class="column has-text-right">
                <a href="#" class="is-primary" @click.prevent="testConnection(n)">
                  <b-icon icon="rocket-launch-outline" /> Test connection
                </a>
              </div>
            </div>
          </div>
        </div>
      </div>

      <b-button @click="addMessenger" icon-left="plus" type="is-primary" data-cy="btn-add-waha">
        {{ $t('globals.buttons.addNew') }}
      </b-button>
    </div>
  </div>
</template>

<script>
export default {
  name: 'WAHASettings',
  props: {
    form: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      data: this.form,
      users: [],
    };
  },
  mounted() {
    if (this.$api && typeof this.$api.queryUsers === 'function') {
      this.$api.queryUsers().then((resp) => {
        this.users = resp || [];
      }).catch(() => {});
    }
  },
  created() {
    this.ensureDefault();
  },
  watch: {
    form: {
      handler() {
        this.data = this.form;
        this.ensureDefault();
      },
      deep: true,
      immediate: true,
    },
  },
  methods: {
    ensureDefault() {
      if (!this.data.waha || !Array.isArray(this.data.waha) || this.data.waha.length === 0) {
        this.$set(this.data, 'waha', []);
        this.addMessenger();
      }
    },
    addMessenger() {
      if (!this.data.waha) {
        this.$set(this.data, 'waha', []);
      }
      this.data.waha.push({
        name: 'whatsapp',
        enabled: true,
        user_id: null,
        user: '',
        root_url: 'http://waha:3000',
        api_key: '',
        session: 'default',
        phone_attribute: 'phone',
        signature: '',
        max_conns: 10,
        max_msg_retries: 2,
        timeout: '10s',
        typing_mode: 'human',
        target_wpm: 60,
        wpm_std: 10,
        keyboard_layout: 'qwerty',
        max_typing_delay_sec: 30,
        messages_per_day: 0,
        messages_per_hour: 0,
      });
    },
    removeMessenger(i) {
      this.data.waha.splice(i, 1);
    },
    testConnection(i) {
      const item = this.data.waha[i];
      if (!item || !item.root_url) {
        this.$utils.toast('Please enter a valid WAHA Base URL first', 'is-warning');
        return;
      }
      this.$utils.toast(`Testing connection to WAHA (${item.session || 'default'})...`, 'is-info');
    },
  },
};
</script>
