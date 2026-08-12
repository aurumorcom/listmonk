<template>
  <section class="schedules">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">
          Schedules
          <span v-if="schedules.length">({{ schedules.length }})</span>
        </h1>
      </div>
      <div class="column has-text-right">
        <b-button :to="{ name: 'sequenceScheduleForm', params: { id: 'new' } }" tag="router-link"
          type="is-primary" icon-left="plus" data-cy="btn-new">
          New Schedule
        </b-button>
      </div>
    </header>

    <b-table :data="filteredSchedules" :loading="loading" hoverable checkable :checked-rows.sync="bulk.checked">
      <template #top-left>
        <div class="columns">
          <div class="column is-6">
            <b-field>
              <b-input v-model="searchQuery" placeholder="Search schedules..." icon="magnify" />
            </b-field>
          </div>
        </div>
        <div class="actions" v-if="bulk.checked.length > 0">
          <a class="a" href="#" @click.prevent="deleteSelectedSchedules" data-cy="btn-delete-schedules">
            <b-icon icon="trash-can-outline" size="is-small" /> Delete
          </a>
          <span class="a">{{ bulk.checked.length }} selected</span>
        </div>
      </template>

      <b-table-column v-slot="props" label="Schedule Name" field="name" sortable>
        <router-link :to="{ name: 'sequenceScheduleForm', params: { id: props.row.id } }">
          <strong>{{ props.row.name }}</strong>
          <copy-text :text="props.row.name" hide-text />
        </router-link>
      </b-table-column>

      <b-table-column v-slot="props" label="Time Zone" field="timezone" sortable>
        <b-tag type="is-info is-light">{{ props.row.timezone || 'UTC' }}</b-tag>
      </b-table-column>

      <b-table-column v-slot="props" label="Timezone Overrides">
        <b-tag :type="props.row.use_contact_timezone ? 'is-success is-light' : 'is-light'">
          {{ props.row.use_contact_timezone ? 'Contact Local TZ' : 'Schedule Default' }}
        </b-tag>
      </b-table-column>

      <b-table-column v-slot="props" label="Holidays">
        <b-tag :type="props.row.skip_holidays ? 'is-warning is-light' : 'is-light'">
          {{ props.row.skip_holidays ? 'Skip National Holidays' : 'Include Holidays' }}
        </b-tag>
      </b-table-column>

      <b-table-column v-slot="props" cell-class="actions" width="15%" align="right">
        <div>
          <router-link :to="{ name: 'sequenceScheduleForm', params: { id: props.row.id } }">
            <b-tooltip label="Edit Schedule" type="is-dark">
              <b-icon icon="pencil-outline" size="is-small" />
            </b-tooltip>
          </router-link>
          <a href="#" @click.prevent="deleteSchedule(props.row.id)">
            <b-tooltip label="Delete Schedule" type="is-dark">
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
</template>

<script>
import CopyText from '../components/CopyText.vue';
import EmptyPlaceholder from '../components/EmptyPlaceholder.vue';

export default {
  name: 'Schedules',
  components: {
    CopyText,
    EmptyPlaceholder,
  },
  data() {
    return {
      schedules: [],
      loading: false,
      searchQuery: '',
      bulk: {
        checked: [],
      },
    };
  },
  computed: {
    filteredSchedules() {
      if (!this.searchQuery) return this.schedules;
      const q = this.searchQuery.toLowerCase();
      return this.schedules.filter(
        (s) => s.name.toLowerCase().includes(q) || (s.timezone && s.timezone.toLowerCase().includes(q)),
      );
    },
  },
  mounted() {
    this.loadSchedules();
  },
  methods: {
    loadSchedules() {
      this.loading = true;
      this.$api
        .getSchedules()
        .then((res) => {
          this.schedules = res.data || [];
          this.loading = false;
        })
        .catch(() => {
          this.loading = false;
        });
    },
    deleteSchedule(id) {
      this.$utils.confirm('Delete schedule?', () => {
        this.loading = true;
        this.$api
          .deleteSchedule(id)
          .then(() => {
            this.$utils.toast('Schedule deleted');
            this.loadSchedules();
          })
          .catch(() => {
            this.loading = false;
          });
      });
    },
    deleteSelectedSchedules() {
      this.$utils.confirm(`Delete ${this.bulk.checked.length} selected schedule(s)?`, () => {
        const promises = this.bulk.checked.map((s) => this.$api.deleteSchedule(s.id));
        Promise.all(promises).then(() => {
          this.$utils.toast('Schedules deleted');
          this.bulk.checked = [];
          this.loadSchedules();
        });
      });
    },
  },
};
</script>
