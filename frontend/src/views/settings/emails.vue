<template>
  <div>
    <div class="items mail-servers">
      <div class="block box" v-for="(item, n) in data.smtp" :key="item.uuid || n">
        <div class="columns">
          <div class="column is-2">
            <b-field>
              <b-switch v-model="item.enabled" name="enabled" :native-value="true" data-cy="btn-enable-email">
                {{ $t('globals.buttons.enabled') }}
              </b-switch>
            </b-field>
            <b-field v-if="data.smtp && data.smtp.length > 1">
              <a @click.prevent="$utils.confirm(null, () => removeEmailAccount(n))" href="#" data-cy="btn-delete-email">
                <b-icon icon="trash-can-outline" />
                {{ $t('globals.buttons.delete') }}
              </a>
            </b-field>
          </div><!-- first column -->

          <div class="column" :class="{ disabled: !item.enabled }">
            <div class="columns">
              <div class="column is-9">
                <b-field :label="$t('settings.mailserver.host')" label-position="on-border"
                  :message="$t('settings.mailserver.hostHelp')">
                  <b-input v-model="item.host" name="host" placeholder="smtp.yourmailserver.net" :maxlength="200" />
                </b-field>
              </div>
              <div class="column">
                <b-field :label="$t('settings.mailserver.port')" label-position="on-border"
                  :message="$t('settings.mailserver.portHelp')">
                  <b-numberinput v-model="item.port" name="port" type="is-light" controls-position="compact"
                    placeholder="25" min="1" max="65535" />
                </b-field>
              </div>
            </div><!-- host -->

            <div class="columns">
              <div class="column is-3">
                <b-field :label="$t('settings.mailserver.authProtocol')" label-position="on-border">
                  <b-select v-model="item.auth_protocol" name="auth_protocol" expanded>
                    <option value="login">
                      LOGIN
                    </option>
                    <option value="cram">
                      CRAM
                    </option>
                    <option value="plain">
                      PLAIN
                    </option>
                    <option value="none">
                      None
                    </option>
                  </b-select>
                </b-field>
              </div>
              <div class="column">
                <b-field grouped>
                  <b-field :label="$t('settings.mailserver.username')" label-position="on-border" expanded>
                    <b-input v-model="item.username" :custom-class="`smtp-username-${n}`"
                      :disabled="item.auth_protocol === 'none'" name="username" placeholder="mysmtp" :maxlength="200" />
                  </b-field>
                  <b-field :label="$t('settings.mailserver.password')" label-position="on-border" expanded
                    :message="$t('settings.mailserver.passwordHelp')">
                    <b-input v-model="item.password" :disabled="item.auth_protocol === 'none'" name="password"
                      type="password" :custom-class="`password-${n}`"
                      :placeholder="$t('settings.mailserver.passwordHelp')" :maxlength="200" />
                  </b-field>
                </b-field>
              </div>
            </div><!-- auth -->

            <div class="spaced-links is-size-7">
              <a href="#" @click.prevent="() => fillSettings(n, 'gmail')">Gmail</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'ses')">Amazon SES</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'azure')">Azure ACS</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'mailgun')">Mailgun</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'mailjet')">Mailjet</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'sendgrid')">Sendgrid</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'postmark')">Postmark</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'forwardemail')">Forward Email</a>
              <a href="#" @click.prevent="() => fillSettings(n, 'lettermint')">Lettermint</a>
            </div>
            <hr />

            <div class="columns">
              <div class="column is-6">
                <b-field :label="$t('settings.smtp.heloHost')" label-position="on-border"
                  :message="$t('settings.smtp.heloHostHelp')">
                  <b-input v-model="item.hello_hostname" name="hello_hostname" placeholder="" :maxlength="200" />
                </b-field>
              </div>
              <div class="column">
                <b-field grouped>
                  <b-field :label="$t('settings.mailserver.tls')" expanded :message="$t('settings.mailserver.tlsHelp')"
                    label-position="on-border">
                    <b-select v-model="item.tls_type" name="tls_type">
                      <option value="none">
                        {{ $t('globals.states.off') }}
                      </option>
                      <option value="STARTTLS">
                        STARTTLS
                      </option>
                      <option value="TLS">
                        SSL/TLS
                      </option>
                    </b-select>
                  </b-field>
                  <b-field expanded :message="$t('settings.mailserver.skipTLSHelp')">
                    <b-switch v-model="item.tls_skip_verify" :disabled="item.tls_type === 'none'"
                      name="tls_skip_verify">
                      {{ $t('settings.mailserver.skipTLS') }}
                    </b-switch>
                  </b-field>
                </b-field>
              </div>
            </div><!-- TLS -->
            <hr />

            <div class="columns">
              <div class="column is-4">
                <b-field :label="$t('settings.mailserver.maxConns')" label-position="on-border"
                  :message="$t('settings.mailserver.maxConnsHelp')">
                  <b-numberinput v-model="item.max_conns" name="max_conns" type="is-light" controls-position="compact"
                    placeholder="25" min="1" max="65535" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field :label="$t('settings.mailserver.idleTimeout')" label-position="on-border"
                  :message="$t('settings.mailserver.idleTimeoutHelp')">
                  <b-input v-model="item.idle_timeout" name="idle_timeout" placeholder="15s" :pattern="regDuration"
                    :maxlength="10" @blur="ensureDefault(item, 'idle_timeout', '15s')" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field :label="$t('settings.mailserver.waitTimeout')" label-position="on-border"
                  :message="$t('settings.mailserver.waitTimeoutHelp')">
                  <b-input v-model="item.wait_timeout" name="wait_timeout" placeholder="5s" :pattern="regDuration"
                    :maxlength="10" @blur="ensureDefault(item, 'wait_timeout', '5s')" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-4">
                <b-field :label="$t('settings.smtp.retries')" label-position="on-border"
                  :message="$t('settings.smtp.retriesHelp')">
                  <b-numberinput v-model="item.max_msg_retries" name="max_msg_retries" type="is-light"
                    controls-position="compact" placeholder="2" min="1" max="1000" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field :label="$t('settings.smtp.retryDelay')" label-position="on-border"
                  :message="$t('settings.smtp.retryDelayHelp')">
                  <b-input v-model="item.msg_retry_delay" name="msg_retry_delay" placeholder="0s" :pattern="regDuration"
                    :maxlength="10" @blur="ensureDefault(item, 'msg_retry_delay', '0s')" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field label="Max Send" label-position="on-border" message="Daily max sending quota for this email account (0 = unlimited)">
                  <b-numberinput v-model="item.max_send_per_day" name="max_send_per_day" type="is-light" controls-position="compact" placeholder="0" min="0" max="100000" />
                </b-field>
              </div>
            </div>

            <hr />
            <div class="columns">
              <div class="column is-6">
                <b-field :label="$t('globals.fields.name')" label-position="on-border"
                  :message="$t('settings.mailserver.nameHelp')">
                  <b-input v-model="item.name" name="name" placeholder="email-primary" :maxlength="100" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field :label="$t('settings.smtp.fromAddresses')" label-position="on-border"
                  :message="$t('settings.smtp.fromAddressesHelp')">
                  <b-taginput v-model="item.from_addresses" name="from_addresses" ellipsis icon="tag-outline"
                    :before-adding="validateFromAddress" placeholder="user@example.com, anothersite.com" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column is-12">
                <b-field label="Signature" label-position="on-border" message="Signature appended to cold outreach sequences sent from this email account (supports HTML & Markdown)">
                  <b-input v-model="item.signature" type="textarea" placeholder="Best regards,&#10;John Doe&#10;Account Executive" :rows="3" />
                </b-field>
              </div>
            </div>

            <div class="columns">
              <div class="column">
                <p v-if="(!item.email_headers || item.email_headers.length === 0) && !item.showHeaders">
                  <a href="#" @click.prevent="() => showSMTPHeaders(n)">
                    <b-icon icon="plus" />{{ $t('settings.smtp.setCustomHeaders') }}</a>
                </p>
                <b-field v-if="(item.email_headers && item.email_headers.length > 0) || item.showHeaders" label-position="on-border"
                  :message="$t('settings.smtp.customHeadersHelp')">
                  <b-input v-model="item.strEmailHeaders" name="email_headers" type="textarea"
                    placeholder="[{&quot;X-Custom&quot;: &quot;value&quot;}, {&quot;X-Custom2&quot;: &quot;value&quot;}]" />
                </b-field>
              </div>
            </div>
            <hr />

            <!-- IMAP Settings -->
            <div>
              <p class="is-size-7 has-text-grey mb-3">
                Listmonk monitors this inbox via IMAP to automatically mark contacts as Replied and stop sequences upon response. Leave host empty to disable.
              </p>
              <div class="columns">
                <div class="column is-9">
                  <b-field :label="$t('settings.mailserver.host')" label-position="on-border" message="Incoming mail server (e.g. imap.gmail.com)">
                    <b-input v-model="item.imap_host" name="imap_host" placeholder="imap.yourmailserver.net" :maxlength="200" />
                  </b-field>
                </div>
                <div class="column">
                  <b-field :label="$t('settings.mailserver.port')" label-position="on-border" message="Standard SSL port: 993">
                    <b-numberinput v-model="item.imap_port" name="imap_port" type="is-light" controls-position="compact"
                      placeholder="993" min="1" max="65535" />
                  </b-field>
                </div>
              </div>

              <div class="columns">
                <div class="column is-3">
                  <b-field :label="$t('settings.mailserver.authProtocol')" label-position="on-border">
                    <b-select v-model="item.imap_auth_protocol" name="imap_auth_protocol" expanded>
                      <option value="login">
                        LOGIN
                      </option>
                      <option value="cram">
                        CRAM
                      </option>
                      <option value="plain">
                        PLAIN
                      </option>
                      <option value="none">
                        None
                      </option>
                    </b-select>
                  </b-field>
                </div>
                <div class="column">
                  <b-field grouped>
                    <b-field :label="$t('settings.mailserver.username')" label-position="on-border" expanded>
                      <b-input v-model="item.imap_username" name="imap_username" placeholder="user@domain.com" :maxlength="200" />
                    </b-field>
                    <b-field :label="$t('settings.mailserver.password')" label-position="on-border" expanded message="App-specific password recommended">
                      <b-input v-model="item.imap_password" name="imap_password" type="password" placeholder="••••••••" :maxlength="200" />
                    </b-field>
                  </b-field>
                </div>
              </div>

              <div class="spaced-links is-size-7 mb-3">
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'gmail')">Gmail IMAP</a>
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'outlook')">Outlook IMAP</a>
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'yahoo')">Yahoo IMAP</a>
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'zoho')">Zoho IMAP</a>
              </div>

              <div class="columns">
                <div class="column is-6">
                  <b-field label="Folder" label-position="on-border" message="IMAP inbox folder to monitor (default INBOX)">
                    <b-input v-model="item.imap_folder" name="imap_folder" placeholder="INBOX" :maxlength="100" />
                  </b-field>
                </div>
                <div class="column is-6">
                  <b-field grouped>
                    <b-field :label="$t('settings.mailserver.tls')" expanded :message="$t('settings.mailserver.tlsHelp')" label-position="on-border">
                      <b-select v-model="item.imap_tls_type" name="imap_tls_type">
                        <option value="none">
                          {{ $t('globals.states.off') }}
                        </option>
                        <option value="STARTTLS">
                          STARTTLS
                        </option>
                        <option value="TLS">
                          SSL/TLS
                        </option>
                      </b-select>
                    </b-field>
                    <b-field expanded :message="$t('settings.mailserver.skipTLSHelp')">
                      <b-switch v-model="item.imap_tls_skip_verify" :disabled="item.imap_tls_type === 'none'" name="imap_tls_skip_verify">
                        {{ $t('settings.mailserver.skipTLS') }}
                      </b-switch>
                    </b-field>
                  </b-field>
                </div>
              </div>

              <div class="columns">
                <div class="column is-4">
                  <b-field :label="$t('settings.mailserver.maxConns')" label-position="on-border"
                    :message="$t('settings.mailserver.maxConnsHelp')">
                    <b-numberinput v-model="item.imap_max_conns" name="imap_max_conns" type="is-light"
                      controls-position="compact" placeholder="5" min="1" max="65535" />
                  </b-field>
                </div>
                <div class="column is-4">
                  <b-field :label="$t('settings.mailserver.idleTimeout')" label-position="on-border"
                    :message="$t('settings.mailserver.idleTimeoutHelp')">
                    <b-input v-model="item.imap_idle_timeout" name="imap_idle_timeout" placeholder="15s"
                      :pattern="regDuration" :maxlength="10" @blur="ensureDefault(item, 'imap_idle_timeout', '15s')" />
                  </b-field>
                </div>
                <div class="column is-4">
                  <b-field :label="$t('settings.mailserver.waitTimeout')" label-position="on-border"
                    :message="$t('settings.mailserver.waitTimeoutHelp')">
                    <b-input v-model="item.imap_wait_timeout" name="imap_wait_timeout" placeholder="5s"
                      :pattern="regDuration" :maxlength="10" @blur="ensureDefault(item, 'imap_wait_timeout', '5s')" />
                  </b-field>
                </div>
              </div>

              <div class="columns">
                <div class="column is-4">
                  <b-field :label="$t('settings.smtp.retries')" label-position="on-border"
                    :message="$t('settings.smtp.retriesHelp')">
                    <b-numberinput v-model="item.imap_max_retries" name="imap_max_retries" type="is-light"
                      controls-position="compact" placeholder="3" min="1" max="1000" />
                  </b-field>
                </div>
                <div class="column is-4">
                  <b-field :label="$t('settings.smtp.retryDelay')" label-position="on-border"
                    :message="$t('settings.smtp.retryDelayHelp')">
                    <b-input v-model="item.imap_retry_delay" name="imap_retry_delay" placeholder="30s"
                      :pattern="regDuration" :maxlength="10" @blur="ensureDefault(item, 'imap_retry_delay', '30s')" />
                  </b-field>
                </div>
                <div class="column is-4">
                  <b-field label="Polling Interval" label-position="on-border"
                    message="Time between IMAP reply checks (e.g. 30s)">
                    <b-input v-model="item.imap_interval" name="imap_interval" placeholder="30s"
                      :pattern="regDuration" :maxlength="10" @blur="ensureDefault(item, 'imap_interval', '30s')" />
                  </b-field>
                </div>
              </div>
            </div>

            <hr />

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

            <form @submit.prevent="() => doSMTPTest(item, n)">
              <div class="columns">
                <template v-if="smtpTestItem === n">
                  <div class="column is-5">
                    <strong>{{ $t('settings.general.fromEmail') }}</strong>
                    <br />
                    {{ settings['app.from_email'] }}
                  </div>
                  <div class="column is-4">
                    <b-field :label="$t('settings.smtp.toEmail')" label-position="on-border">
                      <b-input type="email" required v-model="testEmail" :ref="'testEmailTo'"
                        placeholder="email@site.com" :custom-class="`test-email-${n}`" />
                    </b-field>
                  </div>
                </template>
                <div class="column has-text-right">
                  <b-button v-if="smtpTestItem === n" class="is-primary" @click.prevent="() => doSMTPTest(item, n)">
                    {{ $t('settings.smtp.sendTest') }}
                  </b-button>
                  <a href="#" v-else class="is-primary" @click.prevent="showTestForm(n)">
                    <b-icon icon="rocket-launch-outline" /> {{ $t('settings.smtp.testConnection') }}
                  </a>
                </div>
              </div>
              <div v-if="errMsg && smtpTestItem === n">
                <b-field class="mt-4" type="is-danger">
                  <p class="help is-danger">{{ errMsg }}</p>
                </b-field>
              </div>
            </form><!-- smtp test -->
          </div>
        </div><!-- second container column -->
      </div><!-- block -->
    </div><!-- mail-servers -->

    <b-button @click="addEmailAccount" icon-left="plus" type="is-primary">
      {{ $t('globals.buttons.addNew') }}
    </b-button>
  </div>
</template>

<script>
import Vue from 'vue';
import { mapState } from 'vuex';
import { regDuration } from '../../constants';

const smtpTemplates = {
  gmail: {
    host: 'smtp.gmail.com', port: 465, auth_protocol: 'login', tls_type: 'TLS',
  },
  ses: {
    host: 'email-smtp.YOUR-REGION.amazonaws.com', port: 465, auth_protocol: 'login', tls_type: 'TLS',
  },
  azure: {
    host: 'smtp.azurecomm.net', port: 587, auth_protocol: 'login', tls_type: 'STARTTLS',
  },
  mailjet: {
    host: 'in-v3.mailjet.com', port: 465, auth_protocol: 'cram', tls_type: 'TLS',
  },
  mailgun: {
    host: 'smtp.mailgun.org', port: 465, auth_protocol: 'login', tls_type: 'TLS',
  },
  sendgrid: {
    host: 'smtp.sendgrid.net', port: 465, auth_protocol: 'login', tls_type: 'TLS',
  },
  forwardemail: {
    host: 'smtp.forwardemail.net', port: 465, auth_protocol: 'login', tls_type: 'TLS',
  },
  postmark: {
    host: 'smtp.postmarkapp.com', port: 587, auth_protocol: 'cram', tls_type: 'STARTTLS',
  },
  lettermint: {
    host: 'smtp.lettermint.co', port: 465, auth_protocol: 'login', tls_type: 'TLS',
  },
};

const imapTemplates = {
  gmail: {
    imap_host: 'imap.gmail.com', imap_port: 993, imap_auth_protocol: 'login', imap_tls_type: 'TLS',
  },
  outlook: {
    imap_host: 'outlook.office365.com', imap_port: 993, imap_auth_protocol: 'login', imap_tls_type: 'TLS',
  },
  yahoo: {
    imap_host: 'imap.mail.yahoo.com', imap_port: 993, imap_auth_protocol: 'login', imap_tls_type: 'TLS',
  },
  zoho: {
    imap_host: 'imappro.zoho.com', imap_port: 993, imap_auth_protocol: 'login', imap_tls_type: 'TLS',
  },
};

export default Vue.extend({
  name: 'EmailSettings',

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
      smtpTestItem: null,
      testEmail: '',
      errMsg: '',
      regDuration,
    };
  },

  mounted() {
    if (!this.data.smtp) {
      this.$set(this.data, 'smtp', []);
    }
    if (this.data.smtp.length === 0) {
      this.addEmailAccount();
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
        // Ignored
      }
    },

    addEmailAccount() {
      this.data.smtp.push({
        uuid: '',
        enabled: true,
        name: `email-${this.data.smtp.length + 1}`,
        host: '',
        port: 587,
        auth_protocol: 'login',
        username: '',
        password: '',
        hello_hostname: '',
        tls_type: 'STARTTLS',
        tls_skip_verify: false,
        from_addresses: [],
        max_conns: 10,
        idle_timeout: '15s',
        wait_timeout: '5s',
        max_msg_retries: 2,
        msg_retry_delay: '0s',
        max_send_per_day: 0,
        user_id: null,
        signature: '',
        imap_enabled: false,
        imap_host: '',
        imap_port: 993,
        imap_auth_protocol: 'login',
        imap_username: '',
        imap_password: '',
        imap_tls_type: 'TLS',
        imap_tls_skip_verify: false,
        imap_folder: 'INBOX',
        imap_interval: '30s',
        imap_max_conns: 5,
        imap_idle_timeout: '15s',
        imap_wait_timeout: '5s',
        imap_max_retries: 3,
        imap_retry_delay: '30s',
      });
    },

    ensureDefault(obj, prop, def) {
      if (!obj[prop] || !String(obj[prop]).trim()) {
        this.$set(obj, prop, def);
      }
    },

    removeEmailAccount(i) {
      this.data.smtp.splice(i, 1);
    },

    showSMTPHeaders(i) {
      const s = this.data.smtp[i];
      s.showHeaders = true;
      this.data.smtp.splice(i, 1, s);
    },

    doSMTPTest(item) {
      if (!this.testEmail) {
        this.$utils.toast(this.$t('settings.smtp.testEnterEmail'), 'is-danger');
        return;
      }

      this.errMsg = '';
      this.$api.testSMTP({ ...item, email: this.testEmail }).then(() => {
        this.$utils.toast(this.$t('campaigns.testSent'));
      }).catch((err) => {
        if (err.response?.data?.message) {
          this.errMsg = err.response.data.message;
        }
      });
    },

    showTestForm(n) {
      this.smtpTestItem = n;
      this.errMsg = '';
      this.$nextTick(() => {
        const el = document.querySelector(`.test-email-${n}`);
        if (el) el.focus();
      });
    },

    validateFromAddress(v) {
      // Accept an e-mail address (user@example.com) or a domain (example.com).
      return /^[^\s@]+(\.[^\s@]+)+$|^[^\s@]+@[^\s@]+(\.[^\s@]+)+$/.test(v);
    },

    fillSettings(n, key) {
      this.data.smtp.splice(n, 1, {
        ...this.data.smtp[n],
        ...smtpTemplates[key],
        username: '',
        password: '',
        hello_hostname: '',
        tls_skip_verify: false,
      });
    },

    fillIMAPSettings(n, key) {
      this.data.smtp.splice(n, 1, {
        ...this.data.smtp[n],
        ...imapTemplates[key],
      });
    },
  },

  computed: {
    ...mapState(['settings']),
  },
});
</script>
