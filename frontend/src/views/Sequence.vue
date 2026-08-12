<template>
  <section class="sequence">
    <header class="columns page-header">
      <div class="column is-6">
        <p v-if="!isNew" class="tags">
          <b-tag :class="form.status">
            {{ form.status.toUpperCase() }}
          </b-tag>
          <span class="has-text-grey-light is-size-7">
            ID: {{ form.id }}
            UUID: {{ form.uuid }}
          </span>
        </p>
        <h4 class="title is-4">
          {{ isNew ? 'New Sequence' : form.name }}
        </h4>
      </div>

      <div class="column is-6 has-text-right">
        <div class="buttons is-right">
          <b-button @click="() => save()" type="is-primary" icon-left="content-save-outline" :loading="loading" data-cy="btn-save" aria-keyshortcuts="ctrl+s">
            <span class="has-kbd">Save Changes <span class="kbd">Ctrl+S</span></span>
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
              <b-field label="Sequence Name" label-position="on-border">
                <b-input v-model="form.name" required placeholder="Cold Outreach Sequence" :maxlength="200" />
              </b-field>

              <div class="columns">
                <div class="column is-6">
                  <b-field label="Status" label-position="on-border">
                    <b-select v-model="form.status" expanded>
                      <option value="active">Active</option>
                      <option value="paused">Paused</option>
                      <option value="archived">Archived</option>
                    </b-select>
                  </b-field>
                </div>
                <div class="column is-6">
                  <b-field label="Default Sequence Timezone" label-position="on-border" message="Fallback timezone if contact has no tz attribute">
                    <b-autocomplete
                      v-model="form.timezone"
                      :data="filteredTimezones"
                      placeholder="e.g. America/Denver or UTC"
                      open-on-focus
                      clearable
                      expanded
                      @select="option => form.timezone = option || 'UTC'">
                      <template #empty>No timezones found</template>
                    </b-autocomplete>
                  </b-field>
                </div>
              </div>

              <hr />

              <!-- Scheduling Section (Apollo Parity) -->
              <div class="card p-4 mb-4">
                <h5 class="title is-5 mb-3">Scheduling</h5>
                <b-field label="Schedule *" label-position="on-border">
                  <b-select v-model="form.schedule_id" expanded placeholder="Select a schedule...">
                    <option v-for="s in schedules" :key="s.id" :value="s.id">
                      {{ s.name }}
                    </option>
                  </b-select>
                </b-field>

                <div v-if="selectedSchedule" class="content mt-3 p-3 has-background-light rounded">
                  <div v-for="day in daysOfWeek" :key="day.key" class="columns is-mobile is-marginless py-1">
                    <div class="column is-4 py-0">
                      <strong>{{ day.label }}:</strong>
                    </div>
                    <div class="column is-8 py-0">
                      <span v-if="getScheduleDayWindowText(selectedSchedule, day.key)">
                        {{ getScheduleDayWindowText(selectedSchedule, day.key) }}
                      </span>
                      <span v-else class="has-text-grey italic">No sending window set</span>
                    </div>
                  </div>

                  <div class="buttons mt-4">
                    <b-button icon-left="pencil-outline" @click="editSelectedSchedule">Edit schedule</b-button>
                    <b-button icon-left="plus" type="is-light" @click="createNewSchedule">+ Create new schedule</b-button>
                  </div>
                </div>
              </div>

              <b-field v-if="isNew" class="mt-4">
                <b-button type="is-primary" :loading="loading" @click="onContinue" data-cy="btn-continue">
                  Continue
                </b-button>
              </b-field>
            </div>

            <!-- Side Information Box (Matching Campaign.vue) -->
            <div class="column is-4 is-offset-1">
              <br />
              <div class="box">
                <h3 class="title is-size-6">
                  Sequence Overview
                </h3>
                <p class="is-size-7 mb-2"><strong>Timezone:</strong> {{ form.timezone || 'UTC' }}</p>
                <p class="is-size-7 mb-2"><strong>Schedule:</strong> {{ form.send_schedule.enabled ? `${form.send_schedule.start_time} - ${form.send_schedule.end_time}` : '24/7' }}</p>
                <p class="is-size-7 mb-2"><strong>Active Days:</strong> {{ form.send_schedule.enabled ? form.send_schedule.days.join(', ').toUpperCase() : 'All' }}</p>
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

            <!-- Column 1: Step # & Condition -->
            <b-table-column v-slot="props" label="Step & Condition" width="18%">
              <div>
                <b-tag type="is-primary" class="mr-2">Step {{ props.index + 1 }}</b-tag>
                <b-tag :type="getConditionTagType(props.row.condition)">
                  {{ formatCondition(props.row.condition) }}
                </b-tag>
              </div>
            </b-table-column>

            <!-- Column 2: Subject & Content Snippet -->
            <b-table-column v-slot="props" label="Subject Line & Content" width="30%">
              <div>
                <p>
                  <a href="#" @click.prevent="openEditStepCampaign(props.row, props.index)">
                    <strong>{{ props.row.subject || '(No Subject Line)' }}</strong>
                  </a>
                  <copy-text :text="props.row.subject" hide-text />
                </p>
                <p class="is-size-7 has-text-grey" v-if="props.row.body">
                  {{ props.row.body.substring(0, 80) }}{{ props.row.body.length > 80 ? '...' : '' }}
                </p>
              </div>
            </b-table-column>

            <!-- Column 3: Messenger Channel & Delay -->
            <b-table-column v-slot="props" label="Channel & Delay" width="18%">
              <div>
                <b-tag type="is-info is-light" class="mr-2">
                  {{ props.row.messenger === 'whatsapp' || props.row.messenger === 'waha' ? 'WhatsApp' : 'Email' }}
                </b-tag>
                <b-tag type="is-light">
                  {{ props.row.delay_days === 0 ? 'Immediate' : `${props.row.delay_days}d delay` }}
                </b-tag>
              </div>
            </b-table-column>

            <!-- Column 4: Timestamps / Schedule -->
            <b-table-column v-slot="props" label="Schedule Window" width="14%">
              <span class="is-size-7 has-text-grey">
                <b-icon icon="alarm" size="is-small" />
                {{ props.row.delay_days === 0 ? 'Send Immediately' : `Wait ${props.row.delay_days} day(s)` }}
              </span>
            </b-table-column>

            <!-- Column 5: Actions -->
            <b-table-column v-slot="props" cell-class="actions" width="20%" align="right">
              <div>
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
                <b-taginput v-model="selectedMailboxes" :data="availableMailboxes" field="name" placeholder="Select Emails" autocomplete />
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
      selectedMailboxes: [],
      availableMailboxes: [],
      selectedWahaSessions: [],
      availableWahaSessions: [],
      checkedSteps: [],
      form: {
        id: null,
        uuid: '',
        name: '',
        status: 'active',
        schedule_id: null,
        timezone: 'UTC',
        load_balance_mode: 'round_robin',
        send_schedule: {
          enabled: true,
          start_time: '09:00',
          end_time: '17:00',
          days: ['mon', 'tue', 'wed', 'thu', 'fri'],
        },
        mailbox_ids: [],
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
    loadSchedules() {
      this.$api.getSchedules().then((res) => {
        this.schedules = res.data || [];
        if (!this.form.schedule_id && this.schedules.length > 0) {
          this.form.schedule_id = this.schedules[0].id;
        }
      });
    },
    getScheduleDayWindowText(sched, dayKey) {
      if (!sched || !sched.sending_windows) return '';
      let windows = sched.sending_windows;
      if (typeof windows === 'string') {
        try {
          windows = JSON.parse(windows);
        } catch (e) {
          return '';
        }
      }
      const blocks = windows[dayKey];
      if (!blocks || !blocks.length) return '';
      return blocks.map((b) => `${b.start}–${b.end}`).join(' | ');
    },
    editSelectedSchedule() {
      if (this.form.schedule_id) {
        this.$router.push({ name: 'sequenceScheduleForm', params: { id: this.form.schedule_id } });
      }
    },
    createNewSchedule() {
      this.$router.push({ name: 'sequenceScheduleForm', params: { id: 'new' } });
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
        this.save();
      }
    },
    loadSequence(id) {
      this.loading = true;
      this.$api.getSequence(id).then((res) => {
        this.form = { ...this.form, ...res.data };
        if (!this.form.send_schedule || typeof this.form.send_schedule !== 'object') {
          this.form.send_schedule = {
            enabled: true,
            start_time: '09:00',
            end_time: '17:00',
            days: ['mon', 'tue', 'wed', 'thu', 'fri'],
          };
        }
        this.$api.getSequenceSteps(id).then((stepsRes) => {
          this.steps = stepsRes.data.length ? stepsRes.data : this.steps;
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
    onContinue() {
      this.save('continue');
    },
  },
  computed: {
    selectedSchedule() {
      if (!this.form.schedule_id) return null;
      return this.schedules.find((s) => s.id === this.form.schedule_id) || null;
    },
    allTimezones() {
      return this.$utils ? this.$utils.getTimezones() : ['UTC'];
    },
    filteredTimezones() {
      const q = (this.form.timezone || '').toLowerCase();
      return this.allTimezones.filter((tz) => tz.toLowerCase().includes(q));
    },
  },
};
</script>
