<template>
  <section class="schedules">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">
          Schedules
          <span v-if="schedules.length > 0">({{ schedules.length }})</span>
        </h1>
      </div>
      <div class="column has-text-right">
        <b-field expanded>
          <b-button expanded type="is-primary" icon-left="plus" class="btn-new" @click="showNewForm" data-cy="btn-new">
            New Schedule
          </b-button>
        </b-field>
      </div>
    </header>

    <b-table :data="filteredSchedules" :hoverable="true" :loading="loading" default-sort="created_at">
      <b-table-column v-slot="props" field="name" :label="$t('globals.fields.name')" sortable>
        <a href="#" @click.prevent="showEditForm(props.row)">
          {{ props.row.name }}
        </a>
        <b-tag v-if="props.row.isDefault || props.row.is_default">
          {{ $t('templates.default') }}
        </b-tag>
      </b-table-column>

      <b-table-column v-slot="props" field="timezone" label="Timezone" sortable>
        <b-tag class="campaign">
          {{ props.row.timezone || 'UTC' }}
        </b-tag>
      </b-table-column>

      <b-table-column v-slot="props" field="id" :label="$t('globals.fields.id')" sortable>
        {{ props.row.id }}
      </b-table-column>

      <b-table-column v-slot="props" field="created_at" :label="$t('globals.fields.createdAt')" sortable>
        {{ $utils.niceDate(props.row.created_at || props.row.createdAt) }}
      </b-table-column>

      <b-table-column v-slot="props" field="updated_at" :label="$t('globals.fields.updatedAt')" sortable>
        {{ $utils.niceDate(props.row.updated_at || props.row.updatedAt || props.row.created_at || props.row.createdAt) }}
      </b-table-column>

      <b-table-column v-slot="props" cell-class="actions" align="right">
        <div>
          <a href="#" @click.prevent="showEditForm(props.row)" data-cy="btn-edit" :aria-label="$t('globals.buttons.edit')">
            <b-tooltip :label="$t('globals.buttons.edit')" type="is-dark">
              <b-icon icon="pencil-outline" size="is-small" />
            </b-tooltip>
          </a>
          <a
            href="#"
            @click.prevent="$utils.prompt('Clone schedule', { placeholder: 'Name', value: `Copy of ${props.row.name}` }, (name) => cloneSchedule(name, props.row))"
            data-cy="btn-clone"
            :aria-label="$t('globals.buttons.clone')"
          >
            <b-tooltip :label="$t('globals.buttons.clone')" type="is-dark">
              <b-icon icon="file-multiple-outline" size="is-small" />
            </b-tooltip>
          </a>
          <a
            v-if="!props.row.isDefault && !props.row.is_default"
            href="#"
            @click.prevent="$utils.confirm(null, () => makeScheduleDefault(props.row))"
            data-cy="btn-set-default"
            :aria-label="$t('templates.makeDefault')"
          >
            <b-tooltip :label="$t('templates.makeDefault')" type="is-dark">
              <b-icon icon="check-circle-outline" size="is-small" />
            </b-tooltip>
          </a>
          <span v-else class="a has-text-grey-light">
            <b-icon icon="check-circle-outline" size="is-small" />
          </span>

          <a
            v-if="!props.row.isDefault && !props.row.is_default"
            href="#"
            @click.prevent="$utils.confirm(null, () => deleteSchedule(props.row))"
            data-cy="btn-delete"
            :aria-label="$t('globals.buttons.delete')"
          >
            <b-tooltip :label="$t('globals.buttons.delete')" type="is-dark">
              <b-icon icon="trash-can-outline" size="is-small" />
            </b-tooltip>
          </a>
          <span v-else class="a has-text-grey-light">
            <b-icon icon="trash-can-outline" size="is-small" />
          </span>
        </div>
      </b-table-column>

      <template #empty v-if="!loading">
        <empty-placeholder />
      </template>
    </b-table>

    <!-- Modal Form for Schedule Create/Edit -->
    <b-modal scroll="keep" :aria-modal="true" :active.sync="isFormVisible" :width="600" @close="onFormClose">
      <schedule-form v-if="isFormVisible" :data="curItem" :is-editing="isEditing" @finished="formFinished" />
    </b-modal>
  </section>
</template>

<script>
import EmptyPlaceholder from '../components/EmptyPlaceholder.vue';
import ScheduleForm from './ScheduleForm.vue';

export default {
  name: 'Schedules',
  components: {
    EmptyPlaceholder,
    ScheduleForm,
  },
  data() {
    return {
      schedules: [],
      loading: false,
      searchQuery: '',
      curItem: null,
      isEditing: false,
      isFormVisible: false,
    };
  },
  computed: {
    filteredSchedules() {
      if (!this.searchQuery) return this.schedules;
      const q = this.searchQuery.toLowerCase();
      return this.schedules.filter(
        (s) => (s.name && s.name.toLowerCase().includes(q)) || (s.timezone && s.timezone.toLowerCase().includes(q)),
      );
    },
  },
  mounted() {
    if (this.$route.params.id) {
      this.$api.getSchedule(parseInt(this.$route.params.id, 10)).then((data) => {
        this.showEditForm(data);
      });
    } else {
      this.loadSchedules();
    }
  },
  methods: {
    onFormClose() {
      if (this.$route.params.id) {
        this.$router.push({ name: 'sequenceSchedules' });
      }
    },
    loadSchedules() {
      this.loading = true;
      this.$api
        .getSchedules()
        .then((res) => {
          this.schedules = Array.isArray(res) ? res : (res.data || []);
          this.loading = false;
        })
        .catch(() => {
          this.loading = false;
        });
    },
    showNewForm() {
      this.curItem = {
        name: 'Normal Business Hours',
        timezone: 'UTC',
        use_contact_timezone: true,
        skip_holidays: true,
        sending_windows: {
          mon: { start: '08:00', end: '17:00' },
          tue: { start: '08:00', end: '17:00' },
          wed: { start: '08:00', end: '17:00' },
          thu: { start: '08:00', end: '17:00' },
          fri: { start: '08:00', end: '17:00' },
          sat: {},
          sun: {},
        },
      };
      this.isEditing = false;
      this.isFormVisible = true;
    },
    showEditForm(item) {
      this.curItem = JSON.parse(JSON.stringify(item));
      this.isEditing = true;
      this.isFormVisible = true;
    },
    formFinished() {
      this.isFormVisible = false;
      this.loadSchedules();
    },
    makeScheduleDefault(sched) {
      this.$api.setDefaultSchedule(sched.id).then(() => {
        this.$utils.toast(`Schedule '${sched.name}' set as default`);
        this.loadSchedules();
      });
    },
    cloneSchedule(name, sched) {
      const data = {
        ...JSON.parse(JSON.stringify(sched)),
        name,
      };
      delete data.id;
      delete data.uuid;

      this.$api.createSchedule(data).then(() => {
        this.$utils.toast(`Schedule '${name}' created`);
        this.loadSchedules();
      });
    },
    deleteSchedule(sched) {
      this.loading = true;
      this.$api
        .deleteSchedule(sched.id)
        .then(() => {
          this.$utils.toast('Schedule deleted');
          this.loadSchedules();
        })
        .catch(() => {
          this.loading = false;
        });
    },
  },
};
</script>
