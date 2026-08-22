<template>
  <div class="campaign-steps-editor">
    <div class="columns page-header mb-4">
      <div class="column is-8">
        <h3 class="title is-5">
          Campaign Sequence Steps ({{ steps.length }})
        </h3>
        <p class="subtitle is-6 has-text-grey">
          Configure multi-step drip messages, delays, and channel routing.
        </p>
      </div>
      <div class="column has-text-right">
        <b-button
          type="is-primary"
          icon-left="plus"
          @click="addStep"
          :disabled="loading"
        >
          Add Step
        </b-button>
        <b-button
          type="is-success"
          icon-left="content-save-outline"
          class="ml-2"
          @click="saveSteps"
          :loading="saving"
          :disabled="loading"
        >
          Save Steps
        </b-button>
      </div>
    </div>

    <b-loading :active="loading" :is-full-page="false" />

    <div v-if="!loading && steps.length === 0" class="box has-text-centered p-6">
      <b-icon icon="tray-alert" size="is-large" type="is-grey-light" />
      <p class="is-size-5 mt-2">No steps in this campaign sequence yet.</p>
      <p class="has-text-grey is-size-7 mb-4">Click 'Add Step' to create your first drip step.</p>
      <b-button type="is-primary" icon-left="plus" @click="addStep">
        Add First Step
      </b-button>
    </div>

    <!-- Steps Table -->
    <div v-if="steps.length > 0" class="steps-list">
      <div
        v-for="(step, index) in steps"
        :key="step.id || index"
        class="box mb-4 step-card"
        :class="{ 'is-editing': editingIndex === index }"
      >
        <div class="columns is-vcentered">
          <div class="column is-1 has-text-centered">
            <span class="tag is-primary is-medium is-rounded">#{{ index + 1 }}</span>
          </div>

          <div class="column is-5">
            <p class="has-text-weight-bold">
              {{ step.subject || '(Untitled Step)' }}
            </p>
            <p class="is-size-7 has-text-grey">
              <span class="mr-3">
                <b-icon icon="clock-outline" size="is-small" /> Delay: <strong>{{ step.delay || '0s' }}</strong>
              </span>
              <span>
                <b-icon icon="send-outline" size="is-small" /> Messenger: <strong>{{ step.messenger || 'email' }}</strong>
              </span>
            </p>
          </div>

          <div class="column is-3">
            <b-taglist>
              <b-tag type="is-info" class="is-capitalized">{{ step.content_type || 'richtext' }}</b-tag>
              <b-tag v-if="step.template_id" type="is-light">Tpl #{{ step.template_id }}</b-tag>
            </b-taglist>
          </div>

          <div class="column is-3 has-text-right buttons">
            <b-button
              size="is-small"
              icon-left="arrow-up"
              :disabled="index === 0"
              @click="moveUp(index)"
            />
            <b-button
              size="is-small"
              icon-left="arrow-down"
              :disabled="index === steps.length - 1"
              @click="moveDown(index)"
            />
            <b-button
              size="is-small"
              type="is-info"
              icon-left="pencil"
              @click="toggleEdit(index)"
            />
            <b-button
              size="is-small"
              type="is-danger"
              icon-left="trash-can-outline"
              @click="removeStep(index)"
            />
          </div>
        </div>

        <!-- Inline Step Form -->
        <div v-if="editingIndex === index" class="step-edit-form pt-4 mt-3" style="border-top: 1px solid #eee;">
          <div class="columns">
            <div class="column is-8">
              <b-field label="Subject / Message Title" label-position="on-border">
                <b-input v-model="step.subject" required placeholder="Step subject line..." />
              </b-field>
            </div>
            <div class="column is-2">
              <b-field label="Delay (e.g. 1d, 2h, 0s)" label-position="on-border" message="Wait time after prev step">
                <b-input v-model="step.delay" required placeholder="1d" />
              </b-field>
            </div>
            <div class="column is-2">
              <b-field label="Messenger" label-position="on-border">
                <b-select v-model="step.messenger" expanded>
                  <option value="email">Email</option>
                  <option value="whatsapp">WhatsApp</option>
                </b-select>
              </b-field>
            </div>
          </div>

          <div class="columns">
            <div class="column is-4">
              <b-field label="Content Format" label-position="on-border">
                <b-select v-model="step.content_type" expanded>
                  <option value="richtext">Rich Text (HTML)</option>
                  <option value="html">Raw HTML</option>
                  <option value="markdown">Markdown</option>
                  <option value="plain">Plain Text</option>
                </b-select>
              </b-field>
            </div>
            <div class="column is-4">
              <b-field label="Template ID (Optional)" label-position="on-border">
                <b-input v-model.number="step.template_id" type="number" placeholder="Default template" />
              </b-field>
            </div>
          </div>

          <b-field label="Content Body" label-position="on-border">
            <b-input
              v-model="step.body"
              type="textarea"
              rows="6"
              placeholder="Step message body content..."
              required
            />
          </b-field>

          <b-field label="Plain Text Alternative (Optional)" label-position="on-border">
            <b-input
              v-model="step.altbody"
              type="textarea"
              rows="3"
              placeholder="Plain text fallback..."
            />
          </b-field>

          <div class="has-text-right">
            <b-button type="is-light" @click="editingIndex = null">Done Editing Step</b-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'CampaignStepsEditor',
  props: {
    campaignId: {
      type: [Number, String],
      required: true,
    },
  },
  data() {
    return {
      steps: [],
      loading: false,
      saving: false,
      editingIndex: null,
    };
  },
  mounted() {
    if (this.campaignId && this.campaignId !== 'new') {
      this.getSteps();
    }
  },
  methods: {
    getSteps() {
      this.loading = true;
      this.$api.getCampaignSteps(this.campaignId)
        .then((res) => {
          this.steps = Array.isArray(res) ? res : (res.data || []);
          this.loading = false;
        })
        .catch(() => {
          // Fallback to legacy endpoint if campaign endpoint fails
          this.$api.getSequenceSteps(this.campaignId)
            .then((res) => {
              this.steps = Array.isArray(res) ? res : (res.data || []);
              this.loading = false;
            })
            .catch(() => {
              this.steps = [];
              this.loading = false;
            });
        });
    },

    addStep() {
      const nextNum = this.steps.length + 1;
      this.steps.push({
        campaign_id: parseInt(this.campaignId, 10),
        step_number: nextNum,
        delay: nextNum === 1 ? '0s' : '1d',
        subject: `Step #${nextNum}`,
        body: '',
        altbody: '',
        content_type: 'richtext',
        messenger: 'email',
        template_id: null,
      });
      this.editingIndex = this.steps.length - 1;
    },

    toggleEdit(index) {
      if (this.editingIndex === index) {
        this.editingIndex = null;
      } else {
        this.editingIndex = index;
      }
    },

    removeStep(index) {
      this.$utils.confirm('Remove this step?', () => {
        this.steps.splice(index, 1);
        this.reindexSteps();
        if (this.editingIndex === index) {
          this.editingIndex = null;
        }
      });
    },

    moveUp(index) {
      if (index <= 0) return;
      const item = this.steps.splice(index, 1)[0];
      this.steps.splice(index - 1, 0, item);
      this.reindexSteps();
    },

    moveDown(index) {
      if (index >= this.steps.length - 1) return;
      const item = this.steps.splice(index, 1)[0];
      this.steps.splice(index + 1, 0, item);
      this.reindexSteps();
    },

    reindexSteps() {
      this.steps = this.steps.map((step, idx) => ({
        ...step,
        step_number: idx + 1,
      }));
    },

    saveSteps() {
      this.saving = true;
      this.reindexSteps();

      this.$api.saveCampaignSteps(this.campaignId, { steps: this.steps })
        .then(() => {
          this.saving = false;
          this.$utils.toast('Campaign steps saved successfully');
          this.editingIndex = null;
          this.getSteps();
        })
        .catch((err) => {
          this.saving = false;
          this.$utils.toast(err.message || 'Failed to save campaign steps', 'is-danger');
        });
    },
  },
};
</script>

<style scoped>
.step-card {
  transition: all 0.2s ease;
}
.step-card.is-editing {
  border: 1px solid #3273dc;
  box-shadow: 0 0 8px rgba(50, 115, 220, 0.2);
}
</style>
