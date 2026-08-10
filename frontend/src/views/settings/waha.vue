<template>
  <div>
    <div class="items waha-messengers">
      <div class="block box" v-for="(item, n) in data.waha_messengers" :key="n">
        <b-field>
          <b-switch v-model="item.enabled" name="enabled" :native-value="true">
            {{ $t('globals.buttons.enabled') }}
          </b-switch>
        </b-field>
        <b-field>
          <a @click.prevent="$utils.confirm(null, () => removeMessenger(n))" href="#" class="is-size-7">
            <b-icon icon="trash-can-outline" size="is-small" />
            {{ $t('globals.buttons.delete') }}
          </a>
        </b-field>

        <div :class="{ disabled: !item.enabled }">
          <b-field label="Name" label-position="on-border" message="Unique identifier for this WhatsApp messenger">
            <b-input v-model="item.name" name="name" placeholder="whatsapp-main" :maxlength="200" />
          </b-field>

          <b-field label="WAHA Base URL" label-position="on-border" message="HTTP endpoint of the WAHA container">
            <b-input v-model="item.root_url" name="root_url" placeholder="http://waha:3000" :maxlength="200" expanded type="url" />
          </b-field>

          <b-field label="API Key" label-position="on-border" message="X-Api-Key header authentication">
            <b-input v-model="item.api_key" name="api_key" type="password" placeholder="Secret API key" :maxlength="200" />
          </b-field>

          <b-field label="Session ID" label-position="on-border" message="WAHA WhatsApp session name (default: 'default')">
            <b-input v-model="item.session" name="session" placeholder="default" :maxlength="100" />
          </b-field>

          <b-field label="Phone Attribute Key" label-position="on-border" message="JSON key in subscriber attribs for phone number">
            <b-input v-model="item.phone_attribute" name="phone_attribute" placeholder="phone" :maxlength="100" />
          </b-field>

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

          <!-- eslint-disable-next-line vuejs-accessibility/aria-props -->
          <b-collapse class="card mt-3 mb-3" :open="false" aria-id="advanced-human-typing">
            <template #trigger="props">
              <div class="card-header" role="button" aria-controls="advanced-human-typing" :aria-expanded="props.open">
                <p class="card-header-title is-size-7">
                  <b-icon icon="keyboard-outline" size="is-small" class="mr-1" />
                  Advanced Human Typing Settings (Optional Override)
                </p>
                <a class="card-header-icon" aria-label="Toggle">
                  <b-icon :icon="props.open ? 'chevron-up' : 'chevron-down'" />
                </a>
              </div>
            </template>
            <div class="card-content">
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
            </div>
          </b-collapse>
        </div>
      </div>

      <b-button type="is-primary" icon-left="plus" @click="addMessenger">
        Add WhatsApp (WAHA) Messenger
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
    };
  },
  methods: {
    addMessenger() {
      if (!this.data.waha_messengers) {
        this.$set(this.data, 'waha_messengers', []);
      }
      this.data.waha_messengers.push({
        enabled: true,
        name: `whatsapp-${this.data.waha_messengers.length + 1}`,
        root_url: 'http://waha:3000',
        api_key: '',
        session: 'default',
        phone_attribute: 'phone',
        max_conns: 10,
        max_msg_retries: 2,
        timeout: '10s',
        typing_mode: 'human',
        target_wpm: 60,
        wpm_std: 10,
        keyboard_layout: 'qwerty',
        max_typing_delay_sec: 30,
      });
    },
    removeMessenger(i) {
      this.data.waha_messengers.splice(i, 1);
    },
  },
};
</script>
