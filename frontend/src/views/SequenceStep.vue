<template>
  <section class="sequence-step-campaign">
    <header class="columns page-header">
      <div class="column is-6">
        <p v-if="!isNew" class="tags">
          <b-tag class="running">
            Step #{{ stepNumber }}
          </b-tag>
          <span class="has-text-grey-light is-size-7">
            Sequence ID: {{ sequenceId }}
          </span>
        </p>
        <h4 class="title is-4">
          {{ isNew ? 'New campaign' : (form.name || `Step ${stepNumber}`) }}
        </h4>
      </div>

      <div class="column is-6">
        <div class="buttons is-right">
          <b-button @click="saveStep" type="is-primary" icon-left="content-save-outline" :loading="loading" data-cy="btn-save" aria-keyshortcuts="ctrl+s">
            <span class="has-kbd">Save Changes <span class="kbd">Ctrl+S</span></span>
          </b-button>
        </div>
      </div>
    </header>

    <b-loading :active="loading" />

    <b-tabs type="is-boxed" :animated="false" v-model="activeTab" @input="onTab">
      <!-- TAB 1: Step Setup (Campaign Tab Copy) -->
      <b-tab-item label="Campaign" label-position="on-border" value="campaign" icon="rocket-launch-outline">
        <section class="wrap">
          <div class="columns">
            <div class="column is-7">
              <form @submit.prevent="saveStep">
                <b-field label="Name" label-position="on-border">
                  <b-input v-model="form.name" name="name" required placeholder="Name" :maxlength="200" ref="focus" autofocus />
                </b-field>

                <!-- Requirement 1: Subject is ONLY editable when messenger is email -->
                <b-field v-if="isEmailMessenger" label="Subject" label-position="on-border">
                  <b-input v-model="form.subject" name="subject" required placeholder="Subject" :maxlength="5000" />
                </b-field>

                <!-- Requirements 2 & 2.1: Remove List & From address; in place of list add Condition -->
                <b-field label="Condition" label-position="on-border">
                  <b-select v-model="form.condition" expanded required>
                    <option value="always">Always Send</option>
                    <option value="if_read">If Opened / Read</option>
                    <option value="if_not_read">If NOT Opened</option>
                    <option value="if_clicked">If Link Clicked</option>
                  </b-select>
                </b-field>

                <b-field v-if="isEmailMessenger && stepNumber > 1" label="Email Type" label-position="on-border">
                  <b-select v-model="form.emailType" expanded>
                    <option value="Reply">Reply (Keep In-Reply-To Thread)</option>
                    <option value="New Thread">New Thread (Fresh Subject)</option>
                  </b-select>
                </b-field>

                <div class="columns">
                  <div class="column is-6">
                    <b-field label="Messenger" label-position="on-border">
                      <b-select v-model="form.messenger" name="messenger" required expanded @input="onMessengerChange">
                        <template v-if="emailMessengers.length > 1">
                          <optgroup label="email">
                            <option v-for="m in emailMessengers" :value="m" :key="m">
                              {{ m }}
                            </option>
                          </optgroup>
                        </template>
                        <template v-else>
                          <option value="email">email</option>
                        </template>
                        <option v-for="m in otherMessengers" :value="m" :key="m">{{ m }}</option>
                      </b-select>
                    </b-field>
                  </div>
                  <div class="column is-6">
                    <b-field label="Format" label-position="on-border">
                      <b-select v-model="form.content.contentType" expanded>
                        <option v-for="(name, f) in contentTypes" :key="f" :value="f">
                          {{ name }}
                        </option>
                      </b-select>
                    </b-field>
                  </div>
                </div>

                <b-field label="Tags" label-position="on-border">
                  <b-taginput v-model="form.tags" name="tags" ellipsis icon="tag-outline" placeholder="Tags" />
                </b-field>
                <hr />

                <!-- Requirement 3: Send later -> Send delay -->
                <div class="columns">
                  <div class="column is-4">
                    <b-field label="Send delay" data-cy="btn-send-delay">
                      <b-switch v-model="form.sendDelay" />
                    </b-field>
                  </div>
                  <div class="column is-4" v-if="form.sendDelay">
                    <b-field label="Duration" label-position="on-border" message="Duration (e.g. 45s, 15m, 2h, 1d)">
                      <b-input v-model="form.delayDuration" name="delay_duration" placeholder="1d" :pattern="regDuration" :maxlength="10" required />
                    </b-field>
                  </div>
                </div>

                <div>
                  <p class="has-text-right">
                    <a href="#" @click.prevent="onShowHeaders" data-cy="btn-headers">
                      <b-icon icon="plus" />Set custom headers
                    </a>
                  </p>
                  <b-field v-if="form.headersStr !== '[]' || isHeadersVisible" label-position="on-border" message="Custom JSON headers passed during message dispatch">
                    <b-input v-model="form.headersStr" name="headers" type="textarea" placeholder="[{&quot;X-Custom&quot;: &quot;value&quot;}]" />
                  </b-field>
                </div>
                <hr />

                <b-field>
                  <b-button native-type="submit" type="is-primary" :loading="loading" data-cy="btn-save">
                    Save Step Changes
                  </b-button>
                </b-field>
              </form>
            </div>

            <!-- Side Information Box / Send Test Message (Matching Campaign.vue) -->
            <div class="column is-4 is-offset-1">
              <br />
              <div class="box">
                <h3 class="title is-size-6">
                  {{ $t('campaigns.sendTest') }}
                </h3>
                <b-field :message="isWhatsApp ? 'Hit Enter after typing a phone number to add multiple recipients. The numbers must belong to existing subscribers.' : $t('campaigns.sendTestHelp')">
                  <b-taginput v-model="form.testEmails" :before-adding="validateTestRecipient" ellipsis
                    :icon="testIcon" :placeholder="testPlaceholder" />
                </b-field>
                <b-field>
                  <b-button @click="sendTestMessage" :loading="loading"
                    type="is-primary" :icon-left="testIcon">
                    {{ $t('campaigns.send') }}
                  </b-button>
                </b-field>
              </div>
            </div>
          </div>
        </section>
      </b-tab-item>

      <!-- TAB 2: Content (True copy of Campaign Content tab) -->
      <b-tab-item label="Content" icon="text" value="content">
        <editor v-model="form.content" :title="form.name || `Step ${stepNumber}`" :templates="templates" :content-types="contentTypes" />

        <div class="columns">
          <div class="column is-6">
            <p v-if="!isAttachFieldVisible" class="is-size-6 has-text-grey">
              <a href="#" @click.prevent="onShowAttachField" data-cy="btn-attach">
                <b-icon icon="file-upload-outline" size="is-small" /> Add attachments
              </a>
            </p>

            <b-field v-if="isAttachFieldVisible" label="Attachments" label-position="on-border" expanded data-cy="media">
              <b-taginput v-model="form.media" name="media" ellipsis icon="tag-outline" ref="media" field="filename" @focus="onOpenAttach" />
            </b-field>
          </div>
          <div class="column has-text-right">
            <a href="https://listmonk.app/docs/templating/#template-expressions" target="_blank" rel="noopener noreferrer">
              <b-icon icon="code" /> Templating reference
            </a>
            <span v-if="form.content.contentType !== 'plain'" class="is-size-6 has-text-grey ml-6">
              <a v-if="form.altbody === null" href="#" @click.prevent="onAddAltBody">
                <b-icon icon="text" size="is-small" /> Add plain text alt
              </a>
              <a v-else href="#" @click.prevent="$utils.confirm(null, onRemoveAltBody)">
                <b-icon icon="trash-can-outline" size="is-small" /> Remove plain text alt
              </a>
            </span>
          </div>
        </div>

        <div v-if="form.content.contentType !== 'plain'" class="alt-body">
          <b-input v-if="form.altbody !== null" v-model="form.altbody" type="textarea" />
        </div>
      </b-tab-item>

      <!-- TAB 3: Attributes (True copy of Campaign Attributes tab) -->
      <b-tab-item label="Attributes" icon="code" value="attribs">
        <section class="wrap">
          <b-field label="Attributes" message="Custom JSON metadata context for step processing" label-position="on-border">
            <b-input v-model="form.attribsStr" type="textarea" rows="15" />
          </b-field>
        </section>
      </b-tab-item>

      <!-- TAB 4: Archive (True copy of Campaign Archive tab) -->
      <b-tab-item label="Archive" icon="newspaper-variant-outline" value="archive">
        <section class="wrap">
          <div class="columns">
            <div class="column is-4">
              <b-field label="Enable public archive page" data-cy="btn-archive" message="Allow public web viewing of this step content">
                <b-switch v-model="form.archive" />
              </b-field>
            </div>
            <div class="column is-8">
              <b-field grouped position="is-right">
                <b-button @click="saveStep" type="is-primary" icon-left="content-save-outline" data-cy="btn-save">
                  Save Changes
                </b-button>
              </b-field>
            </div>
          </div>

          <div class="columns">
            <div class="column is-6">
              <b-field label="Archive Template" label-position="on-border">
                <b-select placeholder="Select template" v-model="form.archiveTemplateId" name="template" :disabled="!form.archive">
                  <template v-for="t in templates">
                    <option v-if="t.type === 'campaign'" :value="t.id" :key="t.id">
                      {{ t.name }}
                    </option>
                  </template>
                </b-select>
              </b-field>
            </div>

            <div class="column is-6">
              <b-field grouped position="is-right">
                <b-field v-if="form.archive">
                  <b-button @click="onToggleArchivePreview" type="is-primary" icon-left="file-find-outline" data-cy="btn-preview">
                    Preview
                  </b-button>
                </b-field>
              </b-field>
            </div>
          </div>

          <b-field label="Archive Slug" label-position="on-border" message="Custom URL slug for step archive">
            <b-input v-model="form.archiveSlug" name="archive_slug" :disabled="!form.archive" :maxlength="200" />
          </b-field>

          <b-field label="Archive Meta (JSON)" message="Custom archive metadata JSON" label-position="on-border">
            <b-input v-model="form.archiveMetaStr" name="archive_meta" type="textarea" :disabled="!form.archive" rows="15" />
          </b-field>
        </section>
      </b-tab-item>
    </b-tabs>

    <b-modal scroll="keep" :aria-modal="true" :active.sync="isAttachModalOpen" :width="900">
      <div class="modal-card content" style="width: auto">
        <section class="modal-card-body">
          <media is-modal @selected="onAttachSelect" />
        </section>
      </div>
    </b-modal>

    <campaign-preview
      v-if="isPreviewingArchive"
      @close="onToggleArchivePreview"
      type="campaign"
      :archive-meta="form.archiveMetaStr"
      :title="form.name"
      :content-type="form.content.contentType"
      :template-id="form.archiveTemplateId"
      is-post
      is-archive
    />
  </section>
</template>

<script>
import htmlToPlainText from 'textversionjs';
import Vue from 'vue';
import { mapState } from 'vuex';
import { regDuration } from '../constants';

import CampaignPreview from '../components/CampaignPreview.vue';
import Editor from '../components/Editor.vue';
import Media from './Media.vue';

export default Vue.extend({
  name: 'SequenceStep',
  components: {
    Editor,
    Media,
    CampaignPreview,
  },
  data() {
    return {
      regDuration,
      contentTypes: Object.freeze({
        richtext: 'Rich text',
        html: 'Raw HTML',
        markdown: 'Markdown',
        plain: 'Plain text',
        visual: 'Visual',
      }),
      sequenceId: null,
      stepNumber: 1,
      isNew: false,
      loading: false,
      isHeadersVisible: false,
      isAttachFieldVisible: false,
      isAttachModalOpen: false,
      isPreviewingArchive: false,
      activeTab: 'campaign',
      allSteps: [],
      stepIndex: -1,

      form: {
        name: '',
        subject: '',
        condition: 'always',
        emailType: 'Reply',
        messenger: 'email',
        tags: [],
        headersStr: '[]',
        headers: [],
        attribsStr: '{}',
        attribs: {},
        sendDelay: false,
        delayDuration: '1d',
        delay: '0s',
        content: {
          contentType: 'richtext',
          body: '',
          bodySource: null,
          templateId: null,
        },
        altbody: null,
        media: [],
        archive: false,
        archiveSlug: '',
        archiveTemplateId: null,
        archiveMetaStr: '{}',
        testEmails: [],
      },
    };
  },
  computed: {
    ...mapState(['serverConfig', 'templates']),
    isEmailMessenger() {
      if (!this.form.messenger) return true;
      return this.form.messenger === 'email' || this.form.messenger.startsWith('email-');
    },
    isWhatsApp() {
      return this.form.messenger === 'whatsapp' || this.form.messenger === 'waha' || (this.form.messenger && this.form.messenger.startsWith('whatsapp-'));
    },
    testPlaceholder() {
      return this.isWhatsApp ? 'Phone numbers' : this.$t('campaigns.testEmails');
    },
    testIcon() {
      return this.isWhatsApp ? 'phone' : 'email-outline';
    },
    emailMessengers() {
      const msgs = (this.serverConfig && this.serverConfig.messengers) || ['email'];
      return ['email', ...msgs.filter((m) => m.startsWith('email-'))];
    },
    otherMessengers() {
      const msgs = (this.serverConfig && this.serverConfig.messengers) || [];
      const custom = msgs.filter((m) => m !== 'email' && !m.startsWith('email-'));
      const hasWA = custom.some((m) => m === 'whatsapp' || m === 'waha' || m.startsWith('whatsapp-') || m.startsWith('waha-'));
      if (!hasWA) {
        return ['whatsapp', ...custom];
      }
      return custom;
    },
  },
  mounted() {
    window.addEventListener('keydown', this.onKeyboardShortcut);
    const { sequenceId, stepId } = this.$route.params;
    this.sequenceId = parseInt(sequenceId, 10);

    if (stepId === 'new') {
      this.isNew = true;
    } else {
      this.stepNumber = parseInt(stepId, 10) || 1;
    }

    this.loadTemplates();
    this.loadSteps();
  },
  beforeDestroy() {
    window.removeEventListener('keydown', this.onKeyboardShortcut);
  },
  methods: {
    onKeyboardShortcut(e) {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        this.saveStep();
      }
    },
    onMessengerChange() {
      if (!this.isEmailMessenger) {
        this.form.subject = '';
      }
    },
    onTab(tab) {
      window.history.replaceState({}, '', `#${tab}`);
    },
    onShowHeaders() {
      this.isHeadersVisible = !this.isHeadersVisible;
    },
    onShowAttachField() {
      this.isAttachFieldVisible = true;
      this.$nextTick(() => {
        if (this.$refs.media) {
          this.$refs.media.focus();
        }
      });
    },
    onOpenAttach() {
      this.isAttachModalOpen = true;
    },
    onAttachSelect(o) {
      if (this.form.media.some((m) => m.id === o.id)) {
        return;
      }
      this.form.media.push(o);
    },
    onAddAltBody() {
      this.form.altbody = htmlToPlainText(this.form.content.body);
    },
    onRemoveAltBody() {
      this.form.altbody = null;
    },
    onToggleArchivePreview() {
      this.isPreviewingArchive = !this.isPreviewingArchive;
    },
    loadTemplates() {
      this.$api.getTemplates().then((data) => {
        const list = Array.isArray(data) ? data : (data.data || []);
        if (list.length > 0 && !this.form.content.templateId) {
          const tpl = list.find((i) => i.isDefault === true);
          if (tpl) {
            this.form.content.templateId = tpl.id;
          }
        }
      });
    },
    loadSteps() {
      if (!this.sequenceId) return;
      this.loading = true;
      this.$api.getSequenceSteps(this.sequenceId).then((res) => {
        const list = Array.isArray(res) ? res : (res.data || []);
        this.allSteps = list;
        this.loading = false;

        if (this.isNew) {
          this.stepNumber = this.allSteps.length + 1;
        } else {
          const idx = this.allSteps.findIndex((s, i) => (s.stepNumber || s.step_number || i + 1) === this.stepNumber);
          if (idx > -1) {
            this.stepIndex = idx;
            this.hydrateForm(this.allSteps[idx]);
          }
        }
      }).catch(() => {
        this.loading = false;
      });
    },
    hydrateForm(step) {
      this.form.name = step.name || `Step ${step.stepNumber || step.step_number || this.stepNumber}`;
      this.form.subject = step.subject || '';
      this.form.condition = step.condition || 'always';
      this.form.emailType = step.emailType || step.email_type || 'Reply';
      this.form.messenger = step.messenger || 'email';
      this.form.content.body = step.body || '';
      this.form.content.templateId = step.templateId !== undefined ? step.templateId : (step.template_id || null);

      const d = (step.delay || (step.delay_seconds ? `${step.delay_seconds}s` : '') || '').toString().trim();
      if (d && d !== '0s' && d !== '0' && d !== '0m' && d !== '0h' && d !== '0d') {
        this.form.sendDelay = true;
        this.form.delayDuration = d;
      } else {
        this.form.sendDelay = false;
        this.form.delayDuration = '1d';
      }
      this.form.delay = d || '0s';

      const mediaIds = step.mediaIds || step.media_ids || [];
      if (Array.isArray(mediaIds) && mediaIds.length > 0) {
        this.form.media = mediaIds.map((id) => ({ id, filename: `Media #${id}` }));
        this.isAttachFieldVisible = true;
      }
    },
    saveStep() {
      if (!this.sequenceId) return;

      const delayStr = this.form.sendDelay && this.form.delayDuration ? this.form.delayDuration.trim() : '0s';

      const stepPayload = {
        step_number: this.stepNumber,
        delay: delayStr,
        messenger: this.form.messenger,
        condition: this.form.condition,
        subject: this.isEmailMessenger ? this.form.subject : '',
        body: this.form.content.body || '',
        email_type: this.isEmailMessenger ? this.form.emailType : '',
        template_id: this.form.content.templateId || null,
        media_ids: this.form.media.map((m) => m.id),
      };

      const updatedSteps = this.allSteps.map((s, i) => {
        const sNum = s.stepNumber || s.step_number || i + 1;
        if (sNum === this.stepNumber) {
          return stepPayload;
        }
        return {
          step_number: sNum,
          delay: s.delay || (s.delay_seconds ? `${s.delay_seconds}s` : '0s'),
          messenger: s.messenger || 'email',
          condition: s.condition || 'always',
          subject: s.subject || '',
          body: s.body || '',
          email_type: s.emailType || s.email_type || '',
          template_id: s.templateId !== undefined ? s.templateId : (s.template_id || null),
          media_ids: s.mediaIds || s.media_ids || [],
        };
      });

      if (this.isNew || this.stepIndex === -1) {
        updatedSteps.push(stepPayload);
      }

      this.loading = true;
      this.$api.saveSequenceSteps(this.sequenceId, { steps: updatedSteps }).then(() => {
        this.loading = false;
        this.$utils.toast('Sequence step saved successfully');
        this.$router.push({ name: 'sequence', params: { id: this.sequenceId }, hash: '#steps' });
      }).catch(() => {
        this.loading = false;
      });
    },
    validateTestRecipient(val) {
      if (this.isWhatsApp) {
        return /^\+?[1-9][0-9\s\-()]{6,18}$/.test(val.trim());
      }
      return this.$utils.validateEmail(val);
    },
    sendTestMessage() {
      if (!this.form.testEmails || this.form.testEmails.length === 0) {
        this.$utils.toast(this.isWhatsApp ? 'Please enter test phone numbers' : 'Please enter test email addresses', 'is-warning');
        return;
      }
      this.loading = true;
      this.$api.testSequence(this.sequenceId, {
        step_number: this.stepNumber,
        name: this.form.name || `Step ${this.stepNumber}`,
        subject: this.isEmailMessenger ? this.form.subject : '',
        messenger: this.form.messenger,
        body: this.form.content.body,
        altbody: this.form.altbody,
        content_type: this.form.content.contentType,
        template_id: this.form.content.templateId,
        media: this.form.media ? this.form.media.map((m) => m.id) : [],
        subscribers: this.form.testEmails,
      }).then(() => {
        this.loading = false;
        this.$utils.toast(this.$t('campaigns.testSent'));
      }).catch(() => {
        this.loading = false;
      });
    },
  },
});
</script>
