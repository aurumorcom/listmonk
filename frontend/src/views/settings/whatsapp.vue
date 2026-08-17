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
                <b-field :label="$t('globals.fields.name')" label-position="on-border"
                  message="Optional unique name for the WhatsApp server. Must have the prefix whatsapp-. Setting this allows selection for campaigns (e.g. whatsapp-primary).">
                  <b-input v-model="item.name" name="name" placeholder="whatsapp-primary" :maxlength="100" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field label="Host" label-position="on-border" message="HTTP endpoint of the WAHA container">
                  <b-input v-model="item.host" name="host" placeholder="http://waha:3000" :maxlength="200" expanded type="url" />
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
                <b-field label="Max Send" label-position="on-border" message="Daily max sending quota for this WhatsApp session (0 = unlimited)">
                  <b-numberinput v-model="item.max_send_per_day" name="max_send_per_day" type="is-light" controls-position="compact" placeholder="0" min="0" max="100000" />
                </b-field>
              </div>
            </div>

            <hr />

            <!-- Human Typing Settings -->
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
                <b-field label="Target WPM" label-position="on-border" message="Average Words Per Minute (e.g. 60)">
                  <b-numberinput v-model="item.target_wpm" type="is-light" controls-position="compact" placeholder="60" min="10" max="200" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="WPM Std Dev" label-position="on-border" message="Standard deviation for typing speed variation (e.g. 10)">
                  <b-numberinput v-model="item.wpm_std" type="is-light" controls-position="compact" placeholder="10" min="0" max="50" step="0.5" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-4">
                <b-field label="Keyboard Layout" label-position="on-border" message="Simulated keyboard layout for typos">
                  <b-select v-model="item.keyboard_layout" placeholder="qwerty" expanded>
                    <option value="qwerty">QWERTY</option>
                    <option value="azerty">AZERTY</option>
                  </b-select>
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Max Typing Delay (Sec)" label-position="on-border" message="Maximum seconds to simulate typing (e.g. 30)">
                  <b-numberinput v-model="item.max_typing_delay_sec" type="is-light" controls-position="compact" placeholder="30" min="5" max="300" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-12">
                <b-field label="Signature" label-position="on-border" message="Signature appended to cold outreach sequences sent from this WhatsApp session (supports HTML & Markdown)">
                  <b-input v-model="item.signature" type="textarea" placeholder="Best regards,&#10;John Doe&#10;Account Executive" :rows="3" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-6">
                <b-field label="User" label-position="on-border" message="User who owns this channel for personal outreach sequences">
                  <b-select v-model="item.user_id" placeholder="Select user..." expanded>
                    <option :value="null">&mdash; {{ $t("globals.terms.none") }} &mdash;</option>
                    <option v-for="user in users" :key="user.id" :value="user.id">
                      {{ user.name ? `${user.name} (${user.email || user.username})` : (user.email || user.username) }}
                    </option>
                  </b-select>
                </b-field>
              </div>
            </div>

            <hr />

            <!-- Test Connection Section -->
            <form>
              <div class="columns">
                <template v-if="testItemIndex === n">
                  <div class="column is-5">
                    <p class="is-size-7 has-text-grey mt-2">
                      Send a test WhatsApp message to verify the WAHA connection.
                    </p>
                  </div>
                  <div class="column is-4">
                    <b-field label="Recipient Phone" label-position="on-border">
                      <b-input type="text" required v-model="testPhone" :ref="'testPhoneTo'"
                        placeholder="+14155552671" :custom-class="`test-phone-${n}`" :maxlength="50" />
                    </b-field>
                  </div>
                </template>
                <div class="column has-text-right">
                  <b-button v-if="testItemIndex === n" class="is-primary" @click.prevent="() => doWAHATest(item)">
                    Send Test WhatsApp
                  </b-button>
                  <a href="#" v-else class="is-primary" @click.prevent="showTestForm(n)">
                    <b-icon icon="rocket-launch-outline" /> Test Connection
                  </a>
                </div>
              </div>
              <div v-if="errMsg && testItemIndex === n">
                <b-field class="mt-4" type="is-danger">
                  <p class="help is-danger">{{ errMsg }}</p>
                </b-field>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>

    <div class="buttons">
      <b-button type="is-primary" icon-left="plus" @click="addMessenger" data-cy="btn-add-waha">
        Add WhatsApp Account
      </b-button>
    </div>
  </div>
</template>

<script>
import Vue from 'vue';

export default Vue.extend({
  name: 'WhatsAppSettings',

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
      testItemIndex: null,
      testPhone: '',
      errMsg: '',
    };
  },

  mounted() {
    if (!this.data.waha) {
      this.$set(this.data, 'waha', []);
    }
    if (this.data.waha.length === 0) {
      this.addMessenger();
    }
    this.fetchUsers();
  },

  methods: {
    async fetchUsers() {
      try {
        const res = await this.$api.getUsers();
        if (Array.isArray(res)) {
          this.users = res;
        } else if (res && Array.isArray(res.data)) {
          this.users = res.data;
        } else if (res && Array.isArray(res.results)) {
          this.users = res.results;
        }
      } catch (err) {
        // Ignored if permissions not present
      }
    },

    addMessenger() {
      this.data.waha.push({
        enabled: true,
        name: `whatsapp-${this.data.waha.length + 1}`,
        host: 'http://localhost:3000',
        api_key: '',
        session: 'default',
        phone_attribute: 'phone',
        max_conns: 10,
        max_msg_retries: 2,
        timeout: '10s',
        max_send_per_day: 0,
        typing_mode: 'human',
        target_wpm: 60,
        wpm_std: 10,
        keyboard_layout: 'qwerty',
        max_typing_delay_sec: 30,
        user_id: null,
        signature: '',
      });
    },

    removeMessenger(i) {
      this.data.waha.splice(i, 1);
    },

    showTestForm(n) {
      this.testItemIndex = n;
      this.errMsg = '';
      this.$nextTick(() => {
        const el = document.querySelector(`.test-phone-${n}`);
        if (el) el.focus();
      });
    },

    async doWAHATest(item) {
      if (!this.testPhone) {
        this.$utils.toast('Please enter a recipient phone number for the test message', 'is-danger');
        return;
      }

      this.errMsg = '';
      try {
        await this.$api.testWAHA({
          ...item,
          phone: this.testPhone,
        });
        this.$utils.toast('Test WhatsApp message sent successfully!');
      } catch (err) {
        if (err.response && err.response.data && err.response.data.message) {
          this.errMsg = err.response.data.message;
        } else {
          this.errMsg = err.message || 'Error sending test WhatsApp message';
        }
      }
    },
  },
});
</script>
