<template>
  <section class="sequence">
    <header class="columns page-header">
      <div class="column is-6">
        <p v-if="!isNew" class="tags">
          <b-tag :class="form.status === 'active' ? 'running' : form.status">
            {{ $t(`campaigns.status.${form.status === 'active' ? 'running' : form.status}`) }}
          </b-tag>
          <span class="has-text-grey-light is-size-7">
            ID: {{ form.id }}
            <span v-if="form.uuid" class="ml-2">UUID: {{ form.uuid }}</span>
          </span>
        </p>
        <h4 class="title is-4">
          {{ isNew ? 'New Sequence' : form.name }}
        </h4>
      </div>

      <div class="column is-6 has-text-right" v-if="!isNew">
        <div class="buttons is-right">
          <b-button @click="() => save('save')" type="is-primary" icon-left="content-save-outline" :loading="loading" data-cy="btn-save" aria-keyshortcuts="ctrl+s">
            <span class="has-kbd">Save Changes <span class="kbd">Ctrl+S</span></span>
          </b-button>
          <b-button v-if="form.status === 'paused' || form.status === 'draft'"
            @click="toggleStatus('active')" type="is-primary" icon-left="rocket-launch-outline"
            :loading="loading" data-cy="btn-start">
            Start Sequence
          </b-button>
          <b-button v-else-if="form.status === 'active'" @click="toggleStatus('paused')" type="is-primary" icon-left="pause-circle-outline" :loading="loading" data-cy="btn-pause">
            Pause Sequence
          </b-button>
        </div>
      </div>
    </header>

    <b-loading :active="loading" />

    <b-tabs type="is-boxed" :animated="false" v-model="activeTab" @input="onTab">
      <!-- TAB 1: General & Schedule Settings -->
      <b-tab-item label="Sequence" icon="rocket-launch-outline" value="sequence">
        <section class="wrap">
          <div class="columns">
            <div class="column is-7">
              <form @submit.prevent="onFormSubmit">
                <b-field label="Name" label-position="on-border">
                  <b-input v-model="form.name" required placeholder="Name" :maxlength="200" />
                </b-field>

                <b-field label="Schedule *" label-position="on-border">
                  <b-select v-model="form.schedule_id" expanded placeholder="Select a schedule..." required>
                    <option v-for="s in schedules" :key="s.id" :value="s.id">
                      {{ s.name }}
                    </option>
                  </b-select>
                </b-field>

                <b-field label="Tags" label-position="on-border">
                  <b-taginput v-model="form.tags" name="tags" ellipsis icon="tag-outline" placeholder="Tags" />
                </b-field>

                <div>
                  <p class="has-text-right">
                    <a href="#" @click.prevent="onShowHeaders" data-cy="btn-headers">
                      <b-icon icon="plus" />Set custom headers
                    </a>
                  </p>
                  <b-field v-if="form.headersStr !== '[]' || isHeadersVisible" label-position="on-border"
                    :message="$t('campaigns.customHeadersHelp')">
                    <b-input v-model="form.headersStr" name="headers" type="textarea"
                      placeholder='[{"X-Custom": "value"}, {"X-Custom2": "value"}]' />
                  </b-field>
                </div>

                <hr />

                <b-field v-if="isNew" class="mt-4">
                  <b-button native-type="submit" type="is-primary" :loading="loading" data-cy="btn-continue">
                    Continue
                  </b-button>
                </b-field>
              </form>
            </div>

            <!-- Side Information Box (Matching Campaign.vue) -->
            <div class="column is-4 is-offset-1">
              <br />
              <div class="box">
                <h3 class="title is-size-6">
                  Sequence Overview
                </h3>
                <p class="is-size-7 mb-2"><strong>Timezone:</strong> {{ selectedSchedule ? (selectedSchedule.timezone || 'UTC') : 'UTC' }}</p>
                <p class="is-size-7 mb-2"><strong>Schedule:</strong> {{ selectedSchedule ? selectedSchedule.name : 'None selected' }}</p>
                <p class="is-size-7 mb-2"><strong>Contact TZ Override:</strong> {{ selectedSchedule && selectedSchedule.use_contact_timezone ? 'Enabled' : 'Disabled' }}</p>
                <p class="is-size-7 mb-2"><strong>Skip Holidays:</strong> {{ selectedSchedule && selectedSchedule.skip_holidays ? 'Yes' : 'No' }}</p>
                <p class="is-size-7 mb-2"><strong>Pacing:</strong> Fully Calculative (Auto)</p>
              </div>
            </div>
          </div>
        </section>
      </b-tab-item>

      <!-- TAB 2: Steps Table (Patterned after Campaigns.vue) -->
      <b-tab-item label="Steps" icon="format-list-bulleted-square" value="steps" :disabled="isNew">
        <section class="wrap">
          <header class="columns page-header mb-4">
            <div class="column is-8">
              <h2 class="title is-5">
                Sequence Steps
                <span v-if="steps.length">({{ steps.length }})</span>
              </h2>
            </div>
            <div class="column is-4 has-text-right">
              <b-button type="is-primary" icon-left="plus" @click="openNewStepCampaign" data-cy="btn-new-step">
                New Step
              </b-button>
            </div>
          </header>

          <b-table :data="steps" hoverable checkable :checked-rows.sync="checkedSteps">
            <template #top-left>
              <div class="actions" v-if="checkedSteps.length > 0">
                <a class="a" href="#" @click.prevent="deleteSelectedSteps" data-cy="btn-delete-steps">
                  <b-icon icon="trash-can-outline" size="is-small" /> Delete
                </a>
                <span class="a">
                  {{ checkedSteps.length }} selected
                </span>
              </div>
            </template>

            <!-- Column 1: Status -->
            <b-table-column v-slot="props" cell-class="status" field="status" :label="$t('globals.fields.status')" width="10%" header-class="cy-status">
              <div>
                <p>
                  <a href="#" @click.prevent="toggleStepActive(props.index)">
                    <b-tag :class="isStepActive(props.row) ? 'finished' : 'is-light'">
                      {{ isStepActive(props.row) ? 'Active' : 'Inactive' }}
                    </b-tag>
                  </a>
                </p>
              </div>
            </b-table-column>

            <!-- Column 2: Name -->
            <b-table-column v-slot="props" field="name" :label="$t('globals.fields.name')" width="25%" header-class="cy-name">
              <div>
                <p>
                  <a href="#" @click.prevent="openEditStepCampaign(props.row, props.index)">
                    Step {{ props.row.step_number || (props.index + 1) }}{{ props.row.name ? `: ${props.row.name}` : '' }}
                    <copy-text :text="props.row.name || `Step ${props.row.step_number || (props.index + 1)}`" hide-text />
                  </a>
                </p>
                <p class="is-size-7 has-text-grey" v-if="props.row.subject">
                  <copy-text :text="props.row.subject" />
                </p>
              </div>
            </b-table-column>

            <!-- Column 3: Condition (Formatted exactly like Lists column in Campaigns.vue) -->
            <b-table-column v-slot="props" cell-class="lists" field="condition" label="Condition" width="15%" header-class="cy-condition">
              <ul>
                <li>
                  <a href="#" @click.prevent="openEditStepCampaign(props.row, props.index)">
                    {{ formatCondition(props.row.condition) }}
                  </a>
                </li>
              </ul>
            </b-table-column>

            <!-- Column 3: Timestamps -->
            <b-table-column v-slot="props" field="created_at" :label="$t('campaigns.timestamps')" width="22%" header-class="cy-timestamp">
              <div class="fields timestamps">
                <p>
                  <label for="#">{{ $t('globals.fields.createdAt') }}</label>
                  <span>{{ $utils.niceDate(props.row.created_at || props.row.createdAt || form.created_at || new Date(), true) }}</span>
                </p>
                <p v-if="props.row.updated_at || props.row.updatedAt">
                  <label for="#">Updated</label>
                  <span>{{ $utils.niceDate(props.row.updated_at || props.row.updatedAt, true) }}</span>
                </p>
              </div>
            </b-table-column>

            <!-- Column 4: Stats -->
            <b-table-column v-slot="props" field="stats" :label="$t('campaigns.stats')" width="18%">
              <div class="fields stats">
                <p>
                  <label for="#">{{ $t('campaigns.views') }}</label>
                  <span>{{ $utils.formatNumber(props.row.views || 0) }}</span>
                </p>
                <p>
                  <label for="#">{{ $t('campaigns.clicks') }}</label>
                  <span>{{ $utils.formatNumber(props.row.clicks || 0) }}</span>
                </p>
                <p>
                  <label for="#">{{ $t('campaigns.sent') }}</label>
                  <span>
                    {{ $utils.formatNumber(props.row.sent || 0) }} /
                    {{ $utils.formatNumber(props.row.to_send || props.row.total || 0) }}
                  </span>
                </p>
                <p>
                  <label for="#">{{ $t('globals.terms.bounces') }}</label>
                  <span>
                    <router-link :to="{ name: 'bounces', query: { sequence_id: form.id, step_id: props.row.id || props.row.step_number } }">
                      {{ $utils.formatNumber(props.row.bounces || 0) }}
                    </router-link>
                  </span>
                </p>
              </div>
            </b-table-column>

            <!-- Column 5: Actions -->
            <b-table-column v-slot="props" cell-class="actions" width="20%" align="right">
              <div>
                <a href="#" @click.prevent="toggleStepActive(props.index)" aria-label="Toggle Step Active/Inactive">
                  <b-tooltip :label="isStepActive(props.row) ? 'Inactivate Step' : 'Activate Step'" type="is-dark">
                    <b-icon :icon="isStepActive(props.row) ? 'pause-circle-outline' : 'rocket-launch-outline'" size="is-small" />
                  </b-tooltip>
                </a>
                <router-link :to="{ name: 'sequenceAnalytics', query: { sequence_id: form.id, step_id: props.row.id || props.row.step_number } }">
                  <b-tooltip label="Step Analytics" type="is-dark">
                    <b-icon icon="chart-bar" size="is-small" />
                  </b-tooltip>
                </router-link>
                <a href="#" @click.prevent="moveStepUp(props.index)" v-if="props.index > 0" aria-label="Move Up">
                  <b-tooltip label="Move Up" type="is-dark">
                    <b-icon icon="arrow-up" size="is-small" />
                  </b-tooltip>
                </a>
                <a href="#" @click.prevent="moveStepDown(props.index)" v-if="props.index < steps.length - 1" aria-label="Move Down">
                  <b-tooltip label="Move Down" type="is-dark">
                    <b-icon icon="arrow-down" size="is-small" />
                  </b-tooltip>
                </a>
                <a href="#" @click.prevent="cloneStep(props.index)" aria-label="Clone Step">
                  <b-tooltip label="Clone Step" type="is-dark">
                    <b-icon icon="file-multiple-outline" size="is-small" />
                  </b-tooltip>
                </a>
                <a href="#" @click.prevent="openEditStepCampaign(props.row, props.index)" aria-label="Edit Step Campaign">
                  <b-tooltip label="Edit Step Setup" type="is-dark">
                    <b-icon icon="pencil-outline" size="is-small" />
                  </b-tooltip>
                </a>
                <a href="#" @click.prevent="removeStep(props.index)" aria-label="Delete Step">
                  <b-tooltip label="Delete Step" type="is-dark">
                    <b-icon icon="trash-can-outline" size="is-small" />
                  </b-tooltip>
                </a>
              </div>
            </b-table-column>

            <template #empty>
              <empty-placeholder />
            </template>
          </b-table>
        </section>
      </b-tab-item>

      <!-- TAB 3: Sender Pools -->
      <b-tab-item label="Sender Pools" icon="cog-outline" value="senders" :disabled="isNew">
        <section class="wrap">
          <div class="columns">
            <div class="column is-7">
              <b-field label="Email Accounts Pool" label-position="on-border">
                <b-taginput v-model="selectedEmails" :data="availableEmails" field="name" placeholder="Select Emails" autocomplete />
              </b-field>

              <b-field label="WAHA WhatsApp Sessions Pool" label-position="on-border">
                <b-taginput v-model="selectedWahaSessions" :data="availableWahaSessions" placeholder="Select Sessions" autocomplete />
              </b-field>

              <b-field label="Load Balancing Mode" label-position="on-border">
                <b-select v-model="form.load_balance_mode" expanded>
                  <option value="round_robin">Round Robin Allocation</option>
                  <option value="capacity_weighted">Capacity-Weighted (Daily Limit Remaining)</option>
                </b-select>
              </b-field>
            </div>
          </div>
        </section>
      </b-tab-item>

      <!-- TAB 4: Attributes (JSON) -->
      <b-tab-item label="Attributes" icon="code" value="attribs" :disabled="isNew">
        <section class="wrap">
          <b-field label="Attributes (JSON)" message="Custom JSON metadata for sequence runtime context" label-position="on-border">
            <b-input v-model="form.attribsStr" type="textarea" rows="15" />
          </b-field>
        </section>
      </b-tab-item>
    </b-tabs>
  </section>
</template>

<script>
import CopyText from '../components/CopyText.vue';
import EmptyPlaceholder from '../components/EmptyPlaceholder.vue';

export default {
  name: 'Sequence',
  components: {
    EmptyPlaceholder,
    CopyText,
  },
  data() {
    return {
      isNew: true,
      activeTab: 'sequence',
      loading: false,
      isHeadersVisible: false,
      schedules: [],
      daysOfWeek: [
        { key: 'mon', label: 'Monday' },
        { key: 'tue', label: 'Tuesday' },
        { key: 'wed', label: 'Wednesday' },
        { key: 'thu', label: 'Thursday' },
        { key: 'fri', label: 'Friday' },
        { key: 'sat', label: 'Saturday' },
        { key: 'sun', label: 'Sunday' },
      ],
      selectedEmails: [],
      availableEmails: [],
      selectedWahaSessions: [],
      availableWahaSessions: [],
      checkedSteps: [],
      form: {
        id: null,
        uuid: '',
        name: '',
        status: 'active',
        schedule_id: null,
        tags: [],
        headersStr: '[]',
        load_balance_mode: 'round_robin',
        email_ids: [],
        waha_sessions: [],
        attribsStr: '{}',
      },
      steps: [
        {
          step_number: 1,
          delay_days: 0,
          messenger: 'email',
          condition: 'always',
          subject: '',
          body: '',
          media_ids: [],
        },
      ],
    };
  },
  mounted() {
    window.addEventListener('keydown', this.onKeyboardShortcut);
    if (window.location.hash) {
      this.activeTab = window.location.hash.replace('#', '');
    }

    this.loadSchedules();

    const { id } = this.$route.params;
    if (id && id !== 'new') {
      this.isNew = false;
      this.loadSequence(id);
    }
  },
  beforeDestroy() {
    window.removeEventListener('keydown', this.onKeyboardShortcut);
  },
  methods: {
    isStepActive(step) {
      if (step.status === 'inactive' || step.is_disabled || step.enabled === false) {
        return false;
      }
      return true;
    },
    toggleStepActive(idx) {
      const step = this.steps[idx];
      const active = this.isStepActive(step);
      const newStatus = active ? 'inactive' : 'active';
      this.$set(this.steps[idx], 'status', newStatus);
      this.$set(this.steps[idx], 'enabled', !active);
      this.$set(this.steps[idx], 'is_disabled', active);
      this.saveStepsToBackend();
      this.$utils.toast(`Step #${idx + 1} marked as ${newStatus}`);
    },
    formatStatus(status) {
      if (!status) return 'Finished';
      const key = `campaigns.status.${status}`;
      if (this.$te && this.$te(key)) {
        return this.$t(key);
      }
      return status.charAt(0).toUpperCase() + status.slice(1);
    },
    loadSchedules() {
      this.$api.getSchedules().then((res) => {
        this.schedules = Array.isArray(res) ? res : (res.data || []);
        if (!this.form.schedule_id && this.schedules.length > 0) {
          this.form.schedule_id = this.schedules[0].id;
        }
      });
    },
    onShowHeaders() {
      this.isHeadersVisible = true;
    },
    formatCondition(cond) {
      const map = {
        always: 'Always Send',
        if_read: 'If Opened / Read',
        if_not_read: 'If NOT Opened',
        if_clicked: 'If Link Clicked',
      };
      return map[cond] || cond;
    },
    getConditionTagType(cond) {
      const map = {
        always: 'is-primary is-light',
        if_read: 'is-success is-light',
        if_not_read: 'is-warning is-light',
        if_clicked: 'is-info is-light',
      };
      return map[cond] || 'is-light';
    },
    onTab(tab) {
      window.history.replaceState({}, '', `#${tab}`);
    },
    onKeyboardShortcut(e) {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        this.save('save');
      }
    },
    loadSequence(id) {
      this.loading = true;
      this.$api.getSequence(id).then((res) => {
        const d = res.data || res;
        this.form = { ...this.form, ...d };
        if (typeof this.form.tags === 'string') {
          this.form.tags = this.form.tags ? this.form.tags.split(',') : [];
        }
        if (!this.form.headersStr) {
          this.form.headersStr = '[]';
        }
        this.$api.getSequenceSteps(id).then((stepsRes) => {
          const stepList = Array.isArray(stepsRes) ? stepsRes : (stepsRes.data || []);
          this.steps = stepList.length ? stepList : this.steps;
          this.loading = false;
        }).catch(() => {
          this.loading = false;
        });
      }).catch(() => {
        this.loading = false;
      });
    },
    openNewStepCampaign() {
      if (this.isNew || !this.form.id) {
        this.$utils.toast('Please save the sequence first before adding steps', 'is-warning');
        return;
      }
      this.$router.push({
        name: 'sequenceStepCampaign',
        params: { sequenceId: this.form.id, stepId: 'new' },
      });
    },
    openEditStepCampaign(step, idx) {
      this.$router.push({
        name: 'sequenceStepCampaign',
        params: { sequenceId: this.form.id, stepId: step.step_number || idx + 1 },
      });
    },
    cloneStep(idx) {
      const target = this.steps[idx];
      const cloned = {
        ...JSON.parse(JSON.stringify(target)),
        step_number: this.steps.length + 1,
        delay_days: target.delay_days + 1,
        subject: `Copy of ${target.subject || 'Step'}`,
      };
      this.steps.push(cloned);
      this.saveStepsToBackend();
    },
    removeStep(idx) {
      this.$utils.confirm(`Delete step #${idx + 1}?`, () => {
        this.steps.splice(idx, 1);
        this.reindexSteps();
        this.saveStepsToBackend();
      });
    },
    deleteSelectedSteps() {
      this.$utils.confirm(`Delete ${this.checkedSteps.length} selected step(s)?`, () => {
        this.steps = this.steps.filter((s) => !this.checkedSteps.includes(s));
        this.checkedSteps = [];
        this.reindexSteps();
        this.saveStepsToBackend();
      });
    },
    moveStepUp(idx) {
      if (idx <= 0) return;
      const temp = this.steps[idx];
      this.$set(this.steps, idx, this.steps[idx - 1]);
      this.$set(this.steps, idx - 1, temp);
      this.reindexSteps();
      this.saveStepsToBackend();
    },
    moveStepDown(idx) {
      if (idx >= this.steps.length - 1) return;
      const temp = this.steps[idx];
      this.$set(this.steps, idx, this.steps[idx + 1]);
      this.$set(this.steps, idx + 1, temp);
      this.reindexSteps();
      this.saveStepsToBackend();
    },
    reindexSteps() {
      this.steps = this.steps.map((s, idx) => ({ ...s, step_number: idx + 1 }));
    },
    saveStepsToBackend() {
      if (!this.form.id) return;
      this.$api.saveSequenceSteps(this.form.id, { steps: this.steps }).then(() => {
        this.$utils.toast('Step configuration saved.');
      });
    },
    toggleStatus(newStatus) {
      this.form.status = newStatus;
      this.save('save');
    },
    onFormSubmit() {
      if (this.isNew) {
        this.save('continue');
      } else {
        this.save('save');
      }
    },
    save(mode = 'save') {
      this.loading = true;
      const action = this.isNew
        ? this.$api.createSequence(this.form)
        : this.$api.updateSequence(this.form.id, this.form);

      return action.then((res) => {
        const id = res.id || (res.data && res.data.id) || this.form.id;
        this.form.id = id;
        return this.$api.saveSequenceSteps(id, { steps: this.steps }).then(() => {
          this.loading = false;
          this.isNew = false;
          this.$utils.toast('Sequence saved successfully');

          if (mode === 'continue') {
            this.activeTab = 'steps';
            if (this.$route.params.id === 'new') {
              this.$router.replace({ name: 'sequence', params: { id }, hash: '#steps' });
            } else {
              window.history.replaceState({}, '', '#steps');
            }
          } else {
            this.$router.push({ name: 'sequences' });
          }
        }).catch(() => {
          this.loading = false;
        });
      }).catch(() => {
        this.loading = false;
      });
    },
  },
  computed: {
    selectedSchedule() {
      if (!this.form.schedule_id) return null;
      return this.schedules.find((s) => s.id === this.form.schedule_id) || null;
    },
  },
};
</script>
