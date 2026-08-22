<template>
  <section class="campaign-step">
    <header class="columns page-header">
      <div class="column is-6">
        <p class="tags">
          <b-tag type="is-info">
            Campaign #{{ campaignId }}
          </b-tag>
          <span class="has-text-grey-light is-size-7 ml-2">
            Step #{{ stepNumber }}
          </span>
        </p>
        <h4 class="title is-4">
          {{ form.subject || 'New Step' }}
        </h4>
      </div>

      <div class="column is-6 has-text-right">
        <b-field grouped position="is-right">
          <b-field>
            <b-button
              :to="{ name: 'campaign', params: { id: campaignId }, hash: '#steps' }"
              tag="router-link"
              icon-left="arrow-left"
            >
              Back to Campaign
            </b-button>
          </b-field>
          <b-field>
            <b-button
              @click="saveStep"
              :loading="saving"
              type="is-primary"
              icon-left="content-save-outline"
            >
              Save Step
            </b-button>
          </b-field>
        </b-field>
      </div>
    </header>

    <b-loading :active="loading" />

    <b-tabs type="is-boxed" :animated="false" v-model="activeTab">
      <!-- TAB 1: Step Settings -->
      <b-tab-item label="Step Settings" icon="cog-outline" value="settings">
        <section class="wrap">
          <div class="columns">
            <div class="column is-8">
              <form @submit.prevent="saveStep">
                <b-field label="Subject / Title" label-position="on-border">
                  <b-input
                    v-model="form.subject"
                    name="subject"
                    placeholder="Step subject line..."
                    required
                    autofocus
                  />
                </b-field>

                <div class="columns">
                  <div class="column is-6">
                    <b-field label="Delay" label-position="on-border" message="Wait time after previous step (e.g., 0s, 1d, 2h)">
                      <b-input v-model="form.delay" name="delay" required placeholder="1d" />
                    </b-field>
                  </div>
                  <div class="column is-6">
                    <b-field label="Messenger" label-position="on-border">
                      <b-select v-model="form.messenger" name="messenger" required expanded>
                        <option value="email">Email</option>
                        <option value="whatsapp">WhatsApp</option>
                      </b-select>
                    </b-field>
                  </div>
                </div>

                <div class="columns">
                  <div class="column is-6">
                    <b-field label="Content Format" label-position="on-border">
                      <b-select v-model="form.contentType" name="content_type" required expanded>
                        <option value="richtext">Rich Text (HTML)</option>
                        <option value="html">Raw HTML</option>
                        <option value="markdown">Markdown</option>
                        <option value="plain">Plain Text</option>
                      </b-select>
                    </b-field>
                  </div>
                  <div class="column is-6">
                    <b-field label="Template" label-position="on-border">
                      <b-select v-model="form.templateId" name="template" placeholder="Select template..." expanded>
                        <option :value="null">Default Template</option>
                        <template v-for="t in templates">
                          <option v-if="t.type === 'campaign'" :value="t.id" :key="t.id">
                            {{ t.name }}
                          </option>
                        </template>
                      </b-select>
                    </b-field>
                  </div>
                </div>
              </form>
            </div>

            <!-- Test Send Sidebar -->
            <div class="column is-4">
              <div class="box">
                <h3 class="title is-size-6">Send Test Step</h3>
                <b-field message="Recipient email or phone number">
                  <b-input v-model="testRecipient" placeholder="test@example.com" />
                </b-field>
                <b-field>
                  <b-button
                    @click="sendTest"
                    :loading="testing"
                    type="is-primary"
                    icon-left="send"
                  >
                    Send Test
                  </b-button>
                </b-field>
              </div>
            </div>
          </div>
        </section>
      </b-tab-item><!-- settings -->

      <!-- TAB 2: Content Body -->
      <b-tab-item label="Content Body" icon="text" value="content">
        <section class="wrap">
          <editor
            v-model="editorContent"
            :id="campaignId"
            :title="form.subject"
            :templates="templates"
            :content-types="contentTypes"
          />

          <div class="columns mt-3">
            <div class="column is-6">
              <p v-if="!isAttachFieldVisible" class="is-size-6 has-text-grey">
                <a href="#" @click.prevent="isAttachFieldVisible = true">
                  <b-icon icon="file-upload-outline" size="is-small" /> Add Attachments
                </a>
              </p>
              <b-field v-else label="Attachments" label-position="on-border">
                <b-taginput v-model="form.media" ellipsis icon="tag-outline" field="filename" />
              </b-field>
            </div>
            <div class="column has-text-right">
              <span v-if="form.contentType !== 'plain'" class="is-size-6 has-text-grey">
                <a v-if="form.altbody === null" href="#" @click.prevent="onAddAltBody">
                  <b-icon icon="text" size="is-small" /> Add Plain Text Version
                </a>
                <a v-else href="#" @click.prevent="form.altbody = null">
                  <b-icon icon="trash-can-outline" size="is-small" /> Remove Plain Text Version
                </a>
              </span>
            </div>
          </div>

          <div v-if="form.contentType !== 'plain' && form.altbody !== null" class="mt-3">
            <b-field label="Plain Text Fallback" label-position="on-border">
              <b-input v-model="form.altbody" type="textarea" rows="4" />
            </b-field>
          </div>
        </section>
      </b-tab-item><!-- content -->
    </b-tabs>
  </section>
</template>

<script>
import htmlToPlainText from 'textversionjs';
import Vue from 'vue';
import { mapState } from 'vuex';
import Editor from '../components/Editor.vue';

export default Vue.extend({
  name: 'CampaignStep',

  components: {
    Editor,
  },

  data() {
    return {
      campaignId: null,
      stepId: 'new',
      stepNumber: 1,
      loading: false,
      saving: false,
      testing: false,
      activeTab: 'settings',
      isAttachFieldVisible: false,
      testRecipient: '',

      contentTypes: Object.freeze({
        richtext: 'Rich Text (HTML)',
        html: 'Raw HTML',
        markdown: 'Markdown',
        plain: 'Plain Text',
      }),

      allSteps: [],

      form: {
        id: null,
        campaign_id: null,
        step_number: 1,
        delay: '1d',
        subject: '',
        body: '',
        altbody: null,
        contentType: 'richtext',
        messenger: 'email',
        templateId: null,
        media: [],
      },
    };
  },

  computed: {
    ...mapState(['templates']),

    editorContent: {
      get() {
        return {
          contentType: this.form.contentType,
          body: this.form.body,
          templateId: this.form.templateId,
        };
      },
      set(val) {
        if (val) {
          this.form.contentType = val.contentType || this.form.contentType;
          this.form.body = val.body || '';
          this.form.templateId = val.templateId || null;
        }
      },
    },
  },

  mounted() {
    this.campaignId = parseInt(this.$route.params.campaignId, 10);
    this.stepId = this.$route.params.stepId;
    this.form.campaign_id = this.campaignId;
    this.loadSteps();
  },

  methods: {
    loadSteps() {
      this.loading = true;
      this.$api.getCampaignSteps(this.campaignId).then((res) => {
        this.allSteps = Array.isArray(res) ? res : (res.data || []);
        this.loading = false;

        if (this.stepId !== 'new') {
          const targetNum = parseInt(this.stepId, 10);
          const found = this.allSteps.find((s, idx) => (s.step_number || idx + 1) === targetNum || s.id === targetNum);

          if (found) {
            this.stepNumber = found.step_number || targetNum;
            this.form = {
              ...this.form,
              ...found,
              contentType: found.content_type || found.contentType || 'richtext',
              templateId: found.template_id || found.templateId || null,
            };
          }
        } else {
          this.stepNumber = this.allSteps.length + 1;
          this.form.step_number = this.stepNumber;
          this.form.delay = this.stepNumber === 1 ? '0s' : '1d';
        }
      }).catch(() => {
        this.loading = false;
      });
    },

    onAddAltBody() {
      this.form.altbody = htmlToPlainText(this.form.body || '');
    },

    saveStep() {
      this.saving = true;

      const payloadStep = {
        id: this.form.id || 0,
        campaign_id: this.campaignId,
        step_number: this.stepNumber,
        delay: this.form.delay,
        subject: this.form.subject,
        body: this.form.body,
        altbody: this.form.altbody,
        content_type: this.form.contentType,
        messenger: this.form.messenger,
        template_id: this.form.templateId,
      };

      const updatedSteps = [...this.allSteps];
      if (this.stepId === 'new' || this.stepNumber > updatedSteps.length) {
        updatedSteps.push(payloadStep);
      } else {
        updatedSteps[this.stepNumber - 1] = payloadStep;
      }

      this.$api.saveCampaignSteps(this.campaignId, { steps: updatedSteps }).then(() => {
        this.saving = false;
        this.$utils.toast('Step saved successfully');
        this.$router.push({ name: 'campaign', params: { id: this.campaignId }, hash: '#steps' });
      }).catch((err) => {
        this.saving = false;
        this.$utils.toast(err.message || 'Failed to save step', 'is-danger');
      });
    },

    sendTest() {
      if (!this.testRecipient) {
        this.$utils.toast('Please enter a recipient address', 'is-warning');
        return;
      }
      this.testing = true;
      this.$api.testCampaign({
        id: this.campaignId,
        subscriber_id: 0,
        email: this.testRecipient,
      }).then(() => {
        this.testing = false;
        this.$utils.toast('Test step sent');
      }).catch(() => {
        this.testing = false;
      });
    },
  },
});
</script>
