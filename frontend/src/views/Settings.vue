<template>
  <form @submit.prevent="onSubmit">
    <section class="settings">
      <b-loading :is-full-page="true" v-if="loading.settings || isLoading" active />
      <header class="columns page-header">
        <div class="column is-half">
          <h1 class="title is-4">
            {{ $t('settings.title') }}
            <span class="has-text-grey-light">({{ serverConfig.version }})</span>
          </h1>
        </div>
        <div class="column has-text-right">
          <b-field v-if="$can('settings:manage')" expanded>
            <b-button expanded :disabled="!hasFormChanged" type="is-primary" icon-left="content-save-outline"
              native-type="submit" class="isSaveEnabled" data-cy="btn-save">
              {{ $t('globals.buttons.save') }}
            </b-button>
          </b-field>
        </div>
      </header>
      <hr />

      <section class="wrap settings-wrap" v-if="form">
        <b-tabs class="settings-tabs" vertical :animated="false" v-model="tab">
          <b-tab-item :label="$t('settings.general.name')">
            <general-settings :form="form" :key="key" />
          </b-tab-item><!-- general -->

          <b-tab-item :label="$t('settings.performance.name')">
            <performance-settings :form="form" :key="key" />
          </b-tab-item><!-- performance -->

          <b-tab-item :label="$t('settings.privacy.name')">
            <privacy-settings :form="form" :key="key" />
          </b-tab-item><!-- privacy -->

          <b-tab-item :label="$t('settings.security.name')">
            <security-settings :form="form" :key="key" />
          </b-tab-item><!-- security -->

          <b-tab-item :label="$t('settings.media.title')">
            <media-settings :form="form" :key="key" />
          </b-tab-item><!-- media -->

          <b-tab-item :label="$t('settings.smtp.name')">
            <email-settings :form="form" :key="key" />
          </b-tab-item><!-- mail servers -->

          <b-tab-item :label="$t('settings.bounces.name')">
            <bounce-settings :form="form" :key="key" />
          </b-tab-item><!-- bounces -->

          <b-tab-item :label="$t('settings.messengers.name')">
            <messenger-settings :form="form" :key="key" />
          </b-tab-item><!-- messengers -->

          <b-tab-item label="WhatsApp (WAHA)">
            <whatsapp-settings :form="form" :key="key" />
          </b-tab-item><!-- whatsapp -->

          <b-tab-item label="Webhooks">
            <webhook-settings :form="form" :key="key" />
          </b-tab-item><!-- webhooks -->

          <b-tab-item :label="$t('settings.appearance.name')">
            <appearance-settings :form="form" :key="key" />
          </b-tab-item><!-- appearance -->
        </b-tabs>
      </section>
    </section>
  </form>
</template>

<script>
import Vue from 'vue';
import { mapState } from 'vuex';
import AppearanceSettings from './settings/appearance.vue';
import BounceSettings from './settings/bounces.vue';
import GeneralSettings from './settings/general.vue';
import MediaSettings from './settings/media.vue';
import MessengerSettings from './settings/messengers.vue';
import WhatsappSettings from './settings/whatsapp.vue';
import WebhookSettings from './settings/webhooks.vue';
import PerformanceSettings from './settings/performance.vue';
import PrivacySettings from './settings/privacy.vue';
import SecuritySettings from './settings/security.vue';
import EmailSettings from './settings/emails.vue';

export default Vue.extend({
  components: {
    GeneralSettings,
    PerformanceSettings,
    PrivacySettings,
    SecuritySettings,
    MediaSettings,
    EmailSettings,
    BounceSettings,
    MessengerSettings,
    WhatsappSettings,
    WebhookSettings,
    AppearanceSettings,
  },

  data() {
    return {
      // :key="key" is a ack to re-render child components every time settings
      // is pulled. Otherwise, props don't react.
      key: 0,

      isLoading: false,

      // formCopy is a stringified copy of the original settings against which
      // form is compared to detect changes.
      formCopy: '',
      form: null,
      tab: 0,
    };
  },

  methods: {
    async onSubmit() {
      const form = JSON.parse(JSON.stringify(this.form));

      // SMTP boxes.
      let hasDummy = '';
      for (let i = 0; i < form.smtp.length; i += 1) {
        // trim the host before saving
        form.smtp[i].host = form.smtp[i].host?.trim();

        // If it's the dummy UI password placeholder, ignore it.
        if (this.isDummy(form.smtp[i].password)) {
          form.smtp[i].password = '';
        } else if (this.hasDummy(form.smtp[i].password)) {
          hasDummy = `smtp #${i + 1}`;
        }

        if (this.isDummy(form.smtp[i].imap_password)) {
          form.smtp[i].imap_password = '';
        } else if (this.hasDummy(form.smtp[i].imap_password)) {
          hasDummy = `imap #${i + 1}`;
        }

        if (form.smtp[i].strEmailHeaders && form.smtp[i].strEmailHeaders !== '[]') {
          form.smtp[i].email_headers = JSON.parse(form.smtp[i].strEmailHeaders);
        } else {
          form.smtp[i].email_headers = [];
        }

        // Auto-enable IMAP if host is specified
        form.smtp[i].imap_enabled = Boolean(form.smtp[i].imap_host && form.smtp[i].imap_host.trim());
      }

      // Bounces boxes.
      for (let i = 0; i < form['bounce.mailboxes'].length; i += 1) {
        // trim the host before saving
        form['bounce.mailboxes'][i].host = form['bounce.mailboxes'][i].host?.trim();

        // If it's the dummy UI password placeholder, ignore it.
        if (this.isDummy(form['bounce.mailboxes'][i].password)) {
          form['bounce.mailboxes'][i].password = '';
        } else if (this.hasDummy(form['bounce.mailboxes'][i].password)) {
          hasDummy = `bounce #${i + 1}`;
        }
      }

      if (this.isDummy(form['upload.s3.aws_secret_access_key'])) {
        form['upload.s3.aws_secret_access_key'] = '';
      } else if (this.hasDummy(form['upload.s3.aws_secret_access_key'])) {
        hasDummy = 's3';
      }

      if (this.isDummy(form['bounce.sendgrid_key'])) {
        form['bounce.sendgrid_key'] = '';
      } else if (this.hasDummy(form['bounce.sendgrid_key'])) {
        hasDummy = 'sendgrid';
      }

      if (this.isDummy(form['bounce.azure'].shared_secret)) {
        form['bounce.azure'].shared_secret = '';
      } else if (this.hasDummy(form['bounce.azure'].shared_secret)) {
        hasDummy = 'azure shared secret';
      }

      if (this.isDummy(form['security.captcha'].hcaptcha.secret)) {
        form['security.captcha'].hcaptcha.secret = '';
      } else if (this.hasDummy(form['security.captcha'].hcaptcha.secret)) {
        hasDummy = 'captcha';
      }

      if (this.isDummy(form['security.oidc'].client_secret)) {
        form['security.oidc'].client_secret = '';
      } else if (this.hasDummy(form['security.oidc'].client_secret)) {
        hasDummy = 'oidc';
      }

      if (this.isDummy(form['bounce.postmark'].password)) {
        form['bounce.postmark'].password = '';
      } else if (this.hasDummy(form['bounce.postmark'].password)) {
        hasDummy = 'postmark';
      }

      if (this.isDummy(form['bounce.forwardemail'].key)) {
        form['bounce.forwardemail'].key = '';
      } else if (this.hasDummy(form['bounce.forwardemail'].key)) {
        hasDummy = 'forwardemail';
      }

      if (this.isDummy(form['bounce.lettermint'].key)) {
        form['bounce.lettermint'].key = '';
      } else if (this.hasDummy(form['bounce.lettermint'].key)) {
        hasDummy = 'lettermint';
      }

      for (let i = 0; i < form.messengers.length; i += 1) {
        // If it's the dummy UI password placeholder, ignore it.
        if (this.isDummy(form.messengers[i].password)) {
          form.messengers[i].password = '';
        } else if (this.hasDummy(form.messengers[i].password)) {
          hasDummy = `messenger #${i + 1}`;
        }
      }

      for (let i = 0; i < (form.waha || []).length; i += 1) {
        if (this.isDummy(form.waha[i].api_key)) {
          form.waha[i].api_key = '';
        } else if (this.hasDummy(form.waha[i].api_key)) {
          hasDummy = `waha #${i + 1}`;
        }
      }

      if (hasDummy) {
        this.$utils.toast(this.$t('globals.messages.passwordChangeFull', { name: hasDummy }), 'is-danger');
        return false;
      }

      // Sanitize user_id to null if empty
      for (let i = 0; i < (form.smtp || []).length; i += 1) {
        if (!form.smtp[i].user_id) {
          form.smtp[i].user_id = null;
        }
      }
      for (let i = 0; i < (form.waha || []).length; i += 1) {
        if (!form.waha[i].user_id) {
          form.waha[i].user_id = null;
        }
      }

      // Domain blocklist array from multi-line strings.
      form['privacy.domain_blocklist'] = form['privacy.domain_blocklist'].split('\n').map((v) => v.trim().toLowerCase()).filter((v) => v !== '');
      form['privacy.domain_allowlist'] = form['privacy.domain_allowlist'].split('\n').map((v) => v.trim().toLowerCase()).filter((v) => v !== '');

      this.isLoading = true;
      try {
        const data = await this.$api.updateSettings(form);
        await this.$root.awaitRestart(data);
        this.getSettings();
      } finally {
        this.isLoading = false;
      }

      return false;
    },

    getSettings() {
      this.isLoading = true;
      this.$api.getSettings().then((data) => {
        let d = {};
        try {
          // Create a deep-copy of the settings hierarchy.
          d = JSON.parse(JSON.stringify(data));
        } catch (err) {
          return;
        }

        // Serialize the `email_headers` array map to display on the form.
        for (let i = 0; i < d.smtp.length; i += 1) {
          d.smtp[i].strEmailHeaders = JSON.stringify(d.smtp[i].email_headers, null, 4);
        }

        // Ensure waha exists and has default item if empty
        if (!d.waha || !Array.isArray(d.waha) || d.waha.length === 0) {
          d.waha = [
            {
              name: 'whatsapp',
              enabled: true,
              user_id: null,
              user: '',
              host: 'http://waha:3000',
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
              max_send_per_day: 0,
            },
          ];
        } else {
          d.waha = d.waha.map((w) => {
            const item = { ...w };
            if (item.target_wpm === undefined || item.target_wpm === null) item.target_wpm = 60;
            if (item.wpm_std === undefined || item.wpm_std === null) item.wpm_std = 10;
            if (!item.keyboard_layout) item.keyboard_layout = 'qwerty';
            if (item.max_typing_delay_sec === undefined || item.max_typing_delay_sec === null) item.max_typing_delay_sec = 30;
            if (!item.typing_mode) item.typing_mode = 'human';
            return item;
          });
        }

        // Ensure webhooks exists and has default item if empty
        if (!d.webhooks || !Array.isArray(d.webhooks) || d.webhooks.length === 0) {
          d.webhooks = [
            {
              name: '',
              enabled: true,
              url: '',
              secret: '',
              events: ['subscriber.created', 'subscriber.updated', 'contact.created', 'contact.updated'],
              strHeaders: '',
            },
          ];
        }

        // Domain blocklist array to multi-line string.
        d['privacy.domain_blocklist'] = d['privacy.domain_blocklist'].join('\n');
        d['privacy.domain_allowlist'] = d['privacy.domain_allowlist'].join('\n');

        this.key += 1;
        this.form = d;
        this.formCopy = JSON.stringify(d);

        this.$nextTick(() => {
          this.isLoading = false;
        });
      });
    },

    isDummy(pwd) {
      return !pwd || (pwd.match(/â€¢/g) || []).length === pwd.length;
    },

    hasDummy(pwd) {
      return pwd.includes('â€¢');
    },
  },

  computed: {
    ...mapState(['serverConfig', 'loading']),

    hasFormChanged() {
      if (!this.formCopy) {
        return false;
      }
      return JSON.stringify(this.form) !== this.formCopy;
    },
  },

  beforeRouteLeave(to, from, next) {
    if (this.hasFormChanged) {
      this.$utils.confirm(this.$t('globals.messages.confirmDiscard'), () => next(true));
      return;
    }
    next(true);
  },

  mounted() {
    this.tab = this.$utils.getPref('settings.tab') || 0;
    this.getSettings();
  },

  watch: {
    tab(t) {
      this.$utils.setPref('settings.tab', t);
    },
  },
});
</script>
