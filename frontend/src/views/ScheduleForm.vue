<template>
  <section class="schedule-form">
    <header class="columns page-header">
      <div class="column is-6">
        <p v-if="!isNew" class="tags">
          <b-tag class="is-primary">SCHEDULE</b-tag>
          <span class="has-text-grey-light is-size-7">
            ID: {{ form.id }}
            UUID: {{ form.uuid }}
          </span>
        </p>
        <h4 class="title is-4">
          {{ isNew ? 'New Schedule' : form.name }}
        </h4>
      </div>

      <div class="column is-6 has-text-right">
        <div class="buttons is-right">
          <b-button v-if="!isNew" type="is-danger is-light" icon-left="trash-can-outline" @click="confirmDelete">
            Delete
          </b-button>
          <b-button type="is-primary" icon-left="content-save-outline" :loading="loading" @click="save" data-cy="btn-save">
            <span class="has-kbd">Save Schedule <span class="kbd">Ctrl+S</span></span>
          </b-button>
        </div>
      </div>
    </header>

    <b-loading :active="loading" />

    <section class="wrap">
      <div class="columns">
        <!-- Main Form Column (7) -->
        <div class="column is-7">
          <b-field label="Name:" label-position="on-border">
            <b-input v-model="form.name" required placeholder="Normal Business Hours - IST" />
          </b-field>

          <b-field label="Time Zone:" label-position="on-border">
            <b-autocomplete
              v-model="form.timezone"
              :data="filteredTimezones"
              placeholder="India Standard Time (e.g. Asia/Kolkata or UTC)..."
              open-on-focus
              clearable
              expanded
              @select="opt => form.timezone = opt || 'UTC'"
            >
              <template #empty>No timezones found</template>
            </b-autocomplete>
          </b-field>

          <div class="box mt-4">
            <b-field>
              <b-checkbox v-model="form.use_contact_timezone">
                Use the contact's local time zone instead of the schedule's time zone, if the contact contains location data.
              </b-checkbox>
            </b-field>
            <b-field class="mt-3">
              <b-checkbox v-model="form.skip_holidays">
                Skip the following national holidays: Skip the following national holidays: Labor Day, Independence Day, Memorial Day, Thanksgiving, Christmas Eve, Christmas, New Year's Day
              </b-checkbox>
            </b-field>
          </div>

          <!-- Sending Windows per day -->
          <h5 class="title is-5 mt-5 mb-3">Sending Windows</h5>
          <div v-for="day in daysOfWeek" :key="day.key" class="card mb-3 p-4">
            <div class="level mb-2">
              <div class="level-left">
                <strong>{{ day.label }}:</strong>
              </div>
              <div class="level-right buttons">
                <b-button size="is-small" icon-left="plus" type="is-warning" style="background-color: #e2e822; color: #000;" @click="addTimeBlock(day.key)">
                  Add Time Block
                </b-button>
                <b-button size="is-small" type="is-light" @click="clearDayBlocks(day.key)">
                  Clear
                </b-button>
              </div>
            </div>

            <div v-if="form.sending_windows[day.key] && form.sending_windows[day.key].length">
              <div v-for="(block, bIdx) in form.sending_windows[day.key]" :key="bIdx" class="columns is-mobile is-vcentered mb-1">
                <div class="column is-5">
                  <b-field label="Start Time" label-position="on-border">
                    <b-input v-model="block.start" type="time" placeholder="08:00" />
                  </b-field>
                </div>
                <div class="column is-5">
                  <b-field label="End Time" label-position="on-border">
                    <b-input v-model="block.end" type="time" placeholder="17:00" />
                  </b-field>
                </div>
                <div class="column is-2">
                  <b-button type="is-danger is-text" icon-left="close" @click="removeTimeBlock(day.key, bIdx)" />
                </div>
              </div>
            </div>
            <p v-else class="is-size-7 has-text-grey italic">No sending window set</p>
          </div>
        </div>

        <!-- Side Overview Panel (4) -->
        <div class="column is-4 is-offset-1">
          <br />
          <div class="box">
            <h3 class="title is-size-6">Schedule Overview</h3>
            <p class="is-size-7 mb-2"><strong>Time Zone:</strong> {{ form.timezone || 'UTC' }}</p>
            <p class="is-size-7 mb-2"><strong>Contact Local TZ:</strong> {{ form.use_contact_timezone ? 'Enabled' : 'Disabled' }}</p>
            <p class="is-size-7 mb-2"><strong>Skip Holidays:</strong> {{ form.skip_holidays ? 'Yes' : 'No' }}</p>
            <p class="is-size-7 mb-2"><strong>Active Days:</strong> {{ activeDaysCount }}/7 days</p>
          </div>
        </div>
      </div>
    </section>
  </section>
</template>

<script>
export default {
  name: 'ScheduleForm',
  data() {
    return {
      isNew: true,
      loading: false,
      daysOfWeek: [
        { key: 'mon', label: 'Monday' },
        { key: 'tue', label: 'Tuesday' },
        { key: 'wed', label: 'Wednesday' },
        { key: 'thu', label: 'Thursday' },
        { key: 'fri', label: 'Friday' },
        { key: 'sat', label: 'Saturday' },
        { key: 'sun', label: 'Sunday' },
      ],
      form: {
        id: null,
        uuid: '',
        name: 'Normal Business Hours - IST',
        timezone: 'Asia/Kolkata',
        use_contact_timezone: true,
        skip_holidays: true,
        sending_windows: {
          mon: [{ start: '08:00', end: '17:00' }],
          tue: [{ start: '08:00', end: '17:00' }],
          wed: [{ start: '08:00', end: '17:00' }],
          thu: [{ start: '08:00', end: '17:00' }],
          fri: [{ start: '08:00', end: '17:00' }],
          sat: [],
          sun: [],
        },
      },
    };
  },
  computed: {
    allTimezones() {
      return this.$utils ? this.$utils.getTimezones() : ['UTC'];
    },
    filteredTimezones() {
      const q = (this.form.timezone || '').toLowerCase();
      return this.allTimezones.filter((tz) => tz.toLowerCase().includes(q));
    },
    activeDaysCount() {
      let count = 0;
      this.daysOfWeek.forEach((d) => {
        if (this.form.sending_windows[d.key] && this.form.sending_windows[d.key].length > 0) {
          count += 1;
        }
      });
      return count;
    },
  },
  mounted() {
    window.addEventListener('keydown', this.onKeyboardShortcut);
    const { id } = this.$route.params;
    if (id && id !== 'new') {
      this.isNew = false;
      this.loadSchedule(id);
    }
  },
  beforeDestroy() {
    window.removeEventListener('keydown', this.onKeyboardShortcut);
  },
  methods: {
    onKeyboardShortcut(e) {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        this.save();
      }
    },
    loadSchedule(id) {
      this.loading = true;
      this.$api
        .getSchedule(id)
        .then((res) => {
          const data = res.data || res;
          this.form = {
            ...this.form,
            ...data,
          };
          if (!this.form.sending_windows || typeof this.form.sending_windows !== 'object') {
            this.form.sending_windows = {
              mon: [{ start: '08:00', end: '17:00' }],
              tue: [{ start: '08:00', end: '17:00' }],
              wed: [{ start: '08:00', end: '17:00' }],
              thu: [{ start: '08:00', end: '17:00' }],
              fri: [{ start: '08:00', end: '17:00' }],
              sat: [],
              sun: [],
            };
          }
          this.loading = false;
        })
        .catch(() => {
          this.loading = false;
        });
    },
    addTimeBlock(dayKey) {
      if (!this.form.sending_windows[dayKey]) {
        this.$set(this.form.sending_windows, dayKey, []);
      }
      this.form.sending_windows[dayKey].push({ start: '08:00', end: '17:00' });
    },
    removeTimeBlock(dayKey, index) {
      if (this.form.sending_windows[dayKey]) {
        this.form.sending_windows[dayKey].splice(index, 1);
      }
    },
    clearDayBlocks(dayKey) {
      this.$set(this.form.sending_windows, dayKey, []);
    },
    save() {
      this.loading = true;
      const action = this.isNew
        ? this.$api.createSchedule(this.form)
        : this.$api.updateSchedule(this.form.id, this.form);

      action
        .then(() => {
          this.loading = false;
          this.$utils.toast('Schedule saved successfully');
          this.$router.push({ name: 'sequenceSchedules' });
        })
        .catch(() => {
          this.loading = false;
        });
    },
    confirmDelete() {
      this.$utils.confirm(`Delete schedule ${this.form.name}?`, () => {
        this.loading = true;
        this.$api
          .deleteSchedule(this.form.id)
          .then(() => {
            this.$utils.toast('Schedule deleted');
            this.$router.push({ name: 'sequenceSchedules' });
          })
          .catch(() => {
            this.loading = false;
          });
      });
    },
  },
};
</script>
