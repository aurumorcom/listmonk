<template>
  <form @submit.prevent="onSubmit">
    <div class="modal-card content" style="width: auto">
      <header class="modal-card-head">
        <p v-if="isEditing" class="has-text-grey-light is-size-7">
          {{ $t('globals.fields.id') }}: <copy-text :text="`${form.id || (data && data.id) || ''}`" />
        </p>
        <h4 v-if="isEditing">
          {{ form.name }}
        </h4>
        <h4 v-else>
          New Schedule
        </h4>
      </header>

      <section expanded class="modal-card-body">
        <b-loading :active="loading" :is-full-page="false" />

        <b-field label="Name" label-position="on-border">
          <b-input v-model="form.name" required placeholder="Normal Business Hours - IST" />
        </b-field>

        <b-field label="Time Zone" label-position="on-border">
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
              Use the contact's local time zone instead of the schedule's time zone, if available.
            </b-checkbox>
          </b-field>
          <b-field class="mt-3">
            <b-checkbox v-model="form.skip_holidays">
              Skip national holidays (Labor Day, Independence Day, Thanksgiving, Christmas, New Year's Day)
            </b-checkbox>
          </b-field>
        </div>

        <!-- Sending Windows per day (Single From/To per day) -->
        <h5 class="title is-5 mt-5 mb-3">Sending Windows</h5>
        <div class="box">
          <div v-for="day in daysOfWeek" :key="day.key" class="columns is-mobile is-vcentered mb-2">
            <div class="column is-3">
              <strong>{{ day.label }}:</strong>
            </div>
            <div class="column is-4">
              <b-field label="From" label-position="on-border">
                <b-input v-model="dayTimes[day.key].start" type="time" placeholder="08:00" />
              </b-field>
            </div>
            <div class="column is-4">
              <b-field label="To" label-position="on-border">
                <b-input v-model="dayTimes[day.key].end" type="time" placeholder="17:00" />
              </b-field>
            </div>
            <div class="column is-1 has-text-centered">
              <b-button
                type="is-light"
                size="is-small"
                icon-left="close"
                title="Clear day sequence"
                @click="clearDay(day.key)"
              />
            </div>
          </div>
        </div>
      </section>

      <footer class="modal-card-foot has-text-right">
        <b-button @click="cancel">
          {{ $t('globals.buttons.close') }}
        </b-button>
        <b-button v-if="isEditing" type="is-danger is-light" icon-left="trash-can-outline" @click="confirmDelete" class="mr-auto">
          {{ $t('globals.buttons.delete') }}
        </b-button>
        <b-button native-type="submit" type="is-primary" icon-left="content-save-outline" :loading="loading" data-cy="btn-save">
          {{ $t('globals.buttons.save') }}
        </b-button>
      </footer>
    </div>
  </form>
</template>

<script>
import CopyText from '../components/CopyText.vue';

export default {
  name: 'ScheduleForm',
  components: {
    CopyText,
  },
  props: {
    data: {
      type: Object,
      default: () => null,
    },
    isEditing: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
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
        name: 'Normal Business Hours',
        timezone: 'UTC',
        use_contact_timezone: true,
        skip_holidays: true,
      },
      dayTimes: {
        mon: { start: '08:00', end: '17:00' },
        tue: { start: '08:00', end: '17:00' },
        wed: { start: '08:00', end: '17:00' },
        thu: { start: '08:00', end: '17:00' },
        fri: { start: '08:00', end: '17:00' },
        sat: { start: '', end: '' },
        sun: { start: '', end: '' },
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
  },
  mounted() {
    if (this.$route && this.$route.params && this.$route.params.id) {
      const id = parseInt(this.$route.params.id, 10);
      this.loading = true;
      this.$api.getSchedule(id).then((data) => {
        this.initForm(data);
        this.loading = false;
      }).catch(() => {
        this.loading = false;
      });
    } else {
      this.initForm(this.data);
    }
  },
  methods: {
    initForm(inputData) {
      if (inputData && Object.keys(inputData).length > 0) {
        this.form = {
          id: inputData.id || null,
          uuid: inputData.uuid || '',
          name: inputData.name || 'Normal Business Hours',
          timezone: inputData.timezone || 'UTC',
          use_contact_timezone: inputData.use_contact_timezone !== undefined ? inputData.use_contact_timezone : true,
          skip_holidays: inputData.skip_holidays !== undefined ? inputData.skip_holidays : true,
        };

        if (inputData.sending_windows && typeof inputData.sending_windows === 'object') {
          const sw = inputData.sending_windows;
          this.daysOfWeek.forEach((d) => {
            const blocks = sw[d.key];
            if (Array.isArray(blocks) && blocks.length > 0 && blocks[0].start && blocks[0].end) {
              this.dayTimes[d.key].start = blocks[0].start;
              this.dayTimes[d.key].end = blocks[0].end;
            } else {
              this.dayTimes[d.key].start = '';
              this.dayTimes[d.key].end = '';
            }
          });
        }
      }
    },
    clearDay(dayKey) {
      this.dayTimes[dayKey].start = '';
      this.dayTimes[dayKey].end = '';
    },
    cancel() {
      this.$emit('finished');
      if (this.$parent && typeof this.$parent.close === 'function') {
        this.$parent.close();
      } else if (this.$route && this.$route.name === 'sequenceScheduleForm') {
        this.$router.push({ name: 'sequenceSchedules' });
      }
    },
    onSubmit() {
      this.loading = true;

      const sendingWindows = {};
      this.daysOfWeek.forEach((d) => {
        const t = this.dayTimes[d.key];
        if (t.start && t.end) {
          sendingWindows[d.key] = { start: t.start, end: t.end };
        } else {
          sendingWindows[d.key] = {};
        }
      });

      const payload = {
        ...this.form,
        sending_windows: sendingWindows,
      };

      const action = this.isEditing && this.form.id
        ? this.$api.updateSchedule(this.form.id, payload)
        : this.$api.createSchedule(payload);

      action
        .then(() => {
          this.loading = false;
          this.$utils.toast(this.isEditing ? 'Schedule updated successfully' : 'Schedule created successfully');
          this.cancel();
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
            this.loading = false;
            this.$utils.toast('Schedule deleted');
            this.cancel();
          })
          .catch(() => {
            this.loading = false;
          });
      });
    },
  },
};
</script>
