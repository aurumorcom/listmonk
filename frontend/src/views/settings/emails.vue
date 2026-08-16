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
            <!-- Account Identifiers -->
            <div class="columns">
              <div class="column is-6">
                <b-field :label="$t('globals.fields.name')" label-position="on-border" message="Display name or pool identifier">
                  <b-input v-model="item.name" name="name" placeholder="Primary Outreach Account" :maxlength="200" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field :label="$t('settings.smtp.fromAddresses')" label-position="on-border" message="From email address used for outreach & routing">
                  <b-input v-model="item.from_addresses[0]" name="email" placeholder="sales@mycompany.com" :maxlength="200" />
                </b-field>
              </div>
            </div>

            <!-- Section 1: SMTP -->
            <h5 class="title is-6 mb-3 has-text-weight-bold">SMTP</h5>

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
                    <option value="login">LOGIN</option>
                    <option value="cram">CRAM</option>
                    <option value="plain">PLAIN</option>
                    <option value="none">None</option>
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
                      <option value="none">{{ $t('globals.states.off') }}</option>
                      <option value="STARTTLS">STARTTLS</option>
                      <option value="TLS">TLS</option>
                    </b-select>
                  </b-field>
                  <b-field :label="$t('settings.mailserver.skipTLS')" expanded
                    :message="$t('settings.mailserver.skipTLSHelp')" label-position="on-border">
                    <b-switch v-model="item.tls_skip_verify" name="tls_skip_verify" :native-value="true"
                      data-cy="btn-skip-tls-verify" />
                  </b-field>
                </b-field>
              </div>
            </div><!-- tls -->

            <div class="columns">
              <div class="column is-4">
                <b-field :label="$t('settings.smtp.maxConnections')" label-position="on-border"
                  :message="$t('settings.smtp.maxConnectionsHelp')">
                  <b-numberinput v-model="item.max_conns" name="max_conns" type="is-light" controls-position="compact"
                    placeholder="10" min="1" max="10000" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field :label="$t('settings.smtp.retries')" label-position="on-border"
                  :message="$t('settings.smtp.retriesHelp')">
                  <b-numberinput v-model="item.max_msg_retries" name="max_msg_retries" type="is-light"
                    controls-position="compact" placeholder="2" min="0" max="10" />
                </b-field>
              </div>
              <div class="column is-4">
                <b-field :label="$t('settings.smtp.retryDelay')" label-position="on-border"
                  :message="$t('settings.smtp.retryDelayHelp')">
                  <b-input v-model="item.msg_retry_delay" name="msg_retry_delay" placeholder="0s" :pattern="regDuration"
                    :maxlength="20" />
                </b-field>
              </div>
            </div>

            <!-- Standardized Sending Limits Section -->
            <div class="columns">
              <div class="column is-6">
                <b-field label="Daily sending quota" label-position="on-border" message="Daily max sending quota for this email account (0 = unlimited)">
                  <b-numberinput v-model="item.max_send_per_day" name="max_send_per_day" type="is-light" controls-position="compact" placeholder="0" min="0" max="100000" />
                </b-field>
              </div>
              <div class="column is-6">
                <b-field label="Assigned User" label-position="on-border" message="User who owns this channel for personal outreach sequences">
                  <b-select v-model="item.user_id" placeholder="Select assigned user..." expanded>
                    <option :value="null">-- Shared Team Channel (Unassigned) --</option>
                    <option v-for="user in users" :key="user.id" :value="user.id">
                      {{ user.name }} ({{ user.email }})
                    </option>
                  </b-select>
                </b-field>
              </div>
            </div>

            <!-- Dedicated Persona Signature Section -->
            <div class="columns">
              <div class="column is-12">
                <b-field label="Persona Signature (HTML / Markdown)" label-position="on-border" message="Signature appended to cold outreach sequences sent from this email account">
                  <b-input v-model="item.signature" type="textarea" placeholder="Best regards,&#10;John Doe&#10;Account Executive" :rows="3" />
                </b-field>
              </div>
            </div>

            <hr />

            <!-- Section 2: IMAP Inbound Reply Listener -->
            <div class="mb-4">
              <div class="level is-mobile mb-2">
                <div class="level-left">
                  <h5 class="title is-6 mb-0 has-text-weight-bold">IMAP Inbound Reply Tracking</h5>
                </div>
                <div class="level-right">
                  <b-switch v-model="item.imap_enabled" name="imap_enabled" :native-value="true" size="is-small">
                    {{ item.imap_enabled ? $t('globals.buttons.enabled') : 'Disabled' }}
                  </b-switch>
                </div>
              </div>
              <p class="is-size-7 has-text-grey mb-3">
                Listmonk monitors this inbox via IMAP to automatically mark contacts as Replied and stop sequences upon response.
              </p>
            </div>

            <div v-if="item.imap_enabled">
              <div class="columns">
                <div class="column is-9">
                  <b-field label="IMAP Host" label-position="on-border" message="Incoming mail server (e.g. imap.gmail.com)">
                    <b-input v-model="item.imap_host" name="imap_host" placeholder="imap.yourmailserver.net" :maxlength="200" />
                  </b-field>
                </div>
                <div class="column">
                  <b-field label="IMAP Port" label-position="on-border" message="Standard SSL port: 993">
                    <b-numberinput v-model="item.imap_port" name="imap_port" type="is-light" controls-position="compact"
                      placeholder="993" min="1" max="65535" />
                  </b-field>
                </div>
              </div>

              <div class="columns">
                <div class="column is-6">
                  <b-field label="IMAP Username" label-position="on-border" expanded>
                    <b-input v-model="item.imap_username" name="imap_username" placeholder="user@domain.com" :maxlength="200" />
                  </b-field>
                </div>
                <div class="column is-6">
                  <b-field label="IMAP Password" label-position="on-border" expanded message="App-specific password recommended">
                    <b-input v-model="item.imap_password" name="imap_password" type="password" placeholder="••••••••" :maxlength="200" />
                  </b-field>
                </div>
              </div>

              <div class="spaced-links is-size-7 mb-3">
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'gmail')">Gmail IMAP</a>
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'outlook')">Outlook IMAP</a>
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'yahoo')">Yahoo IMAP</a>
                <a href="#" @click.prevent="() => fillIMAPSettings(n, 'zoho')">Zoho IMAP</a>
              </div>
            </div>

            <hr />

            <!-- Test Connection Section -->
            <form>
              <div class="columns">
                <template v-if="smtpTestItem === n">
                  <div class="column is-5">
                    <p class="is-size-7 has-text-grey mt-2">
                      {{ $t('settings.smtp.testHelp') }}
                    </p>
                  </div>
                  <div class="column is-4">
                    <b-field :label="$t('settings.smtp.toEmail')" label-position="on-border">
                      <b-input type="email" required v-model="testEmail" :ref="'testEmailTo'"
                        :placeholder="$t('settings.smtp.toEmailHelp')" :custom-class="`test-email-${n}`" :maxlength="200" />
                    </b-field>
                  </div>
                </template>
                <div class="column has-text-right">
                  <b-button v-if="smtpTestItem === n" class="is-primary" @click.prevent="() => doSMTPTest(item)">
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
            </form>
          </div>
        </div>
      </div>
    </div>

    <div class="buttons">
      <b-button type="is-primary" icon-left="plus" @click="addEmailAccount" data-cy="btn-add-email">
        Add Email Account
      </b-button>
    </div>
  </div>
</template>

<script>
import Vue from 'vue';

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
    host: 'in-v3.mailjet.com', port: 465, auth_protocol: 'login', tls_type: 'TLS',
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
  gmail: { imap_host: 'imap.gmail.com', imap_port: 993, imap_tls_type: 'TLS' },
  outlook: { imap_host: 'outlook.office365.com', imap_port: 993, imap_tls_type: 'TLS' },
  yahoo: { imap_host: 'imap.mail.yahoo.com', imap_port: 993, imap_tls_type: 'TLS' },
  zoho: { imap_host: 'imappro.zoho.com', imap_port: 993, imap_tls_type: 'TLS' },
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
      regDuration: '^(([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+)$',
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
        if (res && res.data && res.data.results) {
          this.users = res.data.results;
        } else if (Array.isArray(res.data)) {
          this.users = res.data;
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
        from_addresses: [''],
        max_conns: 10,
        max_msg_retries: 2,
        msg_retry_delay: '0s',
        max_send_per_day: 0,
        user_id: null,
        signature: '',
        imap_enabled: false,
        imap_host: '',
        imap_port: 993,
        imap_username: '',
        imap_password: '',
        imap_tls_type: 'TLS',
      });
    },

    removeEmailAccount(i) {
      this.data.smtp.splice(i, 1);
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
});
</script>
