<template>
  <section class="sequences">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">
          Sequences
          <span v-if="!isNaN(sequences.length)">({{ filteredSequences.length }})</span>
        </h1>
      </div>
      <div class="column has-text-right">
        <b-field v-if="$can('campaigns:manage')" expanded>
          <b-button expanded :to="{ name: 'sequence', params: { id: 'new' } }" tag="router-link" class="btn-new"
            type="is-primary" icon-left="plus" data-cy="btn-new">
            {{ $t('globals.buttons.new') }}
          </b-button>
        </b-field>
      </div>
    </header>

    <b-table :data="filteredSequences" :loading="loading" :row-class="highlightedRow"
      @check-all="onTableCheck" @check="onTableCheck" :checked-rows.sync="bulk.checked" paginated
      pagination-position="both" :current-page="queryParams.page" :per-page="perPage"
      hoverable checkable @sort="onSort">
      <template #top-left>
        <div class="columns">
          <div class="column is-6">
            <form @submit.prevent>
              <div>
                <b-field>
                  <b-input v-model="searchQuery" name="query" expanded
                    placeholder="Search sequences by name..." icon="magnify" ref="query" />
                  <p class="controls">
                    <b-button native-type="submit" type="is-primary" icon-left="magnify" />
                  </p>
                </b-field>
              </div>
            </form>
          </div>
        </div>

        <div class="actions" v-if="bulk.checked.length > 0">
          <a class="a" href="#" @click.prevent="deleteSelectedSequences" data-cy="btn-delete-sequences">
            <b-icon icon="trash-can-outline" size="is-small" /> Delete
          </a>
          <span class="a">
            {{ numSelectedSequences }} selected
            <span v-if="!bulk.all && filteredSequences.length > perPage">
              &mdash;
              <a href="#" @click.prevent="onSelectAll">
                Select all {{ filteredSequences.length }}
              </a>
            </span>
          </span>
        </div>
      </template>

      <!-- Column 1: Status -->
      <b-table-column v-slot="props" cell-class="status" field="status" :label="$t('globals.fields.status')" width="10%" sortable header-class="cy-status">
        <div>
          <p>
            <router-link :to="{ name: 'sequence', params: { id: props.row.id } }">
              <b-tag :class="props.row.status === 'active' ? 'running' : props.row.status">
                {{ $t(`campaigns.status.${props.row.status === 'active' ? 'running' : props.row.status}`) }}
              </b-tag>
              <span class="spinner is-tiny" v-if="isRunning(props.row.id)">
                <b-loading :is-full-page="false" active />
              </span>
            </router-link>
          </p>
        </div>
      </b-table-column>

      <!-- Column 2: Name -->
      <b-table-column v-slot="props" field="name" :label="$t('globals.fields.name')" width="25%" sortable header-class="cy-name">
        <div>
          <p>
            <router-link :to="{ name: 'sequence', params: { id: props.row.id } }">
              {{ props.row.name }}
              <copy-text :text="props.row.name" hide-text />
            </router-link>
          </p>
          <p class="is-size-7 has-text-grey" v-if="props.row.tags && props.row.tags.length">
            <b-taglist>
              <b-tag class="is-small" v-for="t in props.row.tags" :key="t">
                {{ t }}
              </b-tag>
            </b-taglist>
          </p>
        </div>
      </b-table-column>

      <!-- Column 3: Schedule (Replacing Lists) -->
      <b-table-column v-slot="props" cell-class="lists" field="schedule" label="Schedule" width="15%">
        <ul>
          <li>
            <router-link :to="{ name: 'sequenceSchedules' }">
              {{ getScheduleName(props.row.schedule_id) }}
            </router-link>
          </li>
        </ul>
      </b-table-column>

      <!-- Column 4: Timestamps -->
      <b-table-column v-slot="props" field="created_at" :label="$t('campaigns.timestamps')" width="19%" sortable header-class="cy-timestamp">
        <div class="fields timestamps" :set="stats = getSequenceStats(props.row)">
          <p>
            <label for="#">{{ $t('globals.fields.createdAt') }}</label>
            <span>{{ $utils.niceDate(props.row.created_at || props.row.createdAt, true) }}</span>
          </p>
          <p v-if="stats.started_at || stats.startedAt">
            <label for="#">{{ $t('campaigns.startedAt') }}</label>
            <span>{{ $utils.niceDate(stats.started_at || stats.startedAt, true) }}</span>
          </p>
          <p v-if="stats.ended_at || stats.endedAt || props.row.status === 'finished'">
            <label for="#">{{ $t('campaigns.ended') }}</label>
            <span>{{ $utils.niceDate(stats.ended_at || stats.endedAt || props.row.updated_at, true) }}</span>
          </p>
          <p v-if="(stats.started_at || stats.startedAt) && (stats.ended_at || stats.endedAt)" class="is-capitalized">
            <label for="#"><b-icon icon="alarm" size="is-small" /></label>
            <span>{{ $utils.duration(stats.started_at || stats.startedAt, stats.ended_at || stats.endedAt) }}</span>
          </p>
        </div>
      </b-table-column>

      <!-- Column 5: Stats -->
      <b-table-column v-slot="props" field="stats" :label="$t('campaigns.stats')" width="15%">
        <div class="fields stats" :set="stats = getSequenceStats(props.row)">
          <p>
            <label for="#">{{ $t('campaigns.views') }}</label>
            <span>{{ $utils.formatNumber(stats.views || 0) }}</span>
          </p>
          <p>
            <label for="#">{{ $t('campaigns.clicks') }}</label>
            <span>{{ $utils.formatNumber(stats.clicks || 0) }}</span>
          </p>
          <p>
            <label for="#">{{ $t('campaigns.sent') }}</label>
            <span>
              {{ $utils.formatNumber(stats.sent || 0) }} /
              {{ $utils.formatNumber(stats.to_send || stats.toSend || stats.total || 0) }}
            </span>
          </p>
          <p>
            <label for="#">{{ $t('globals.terms.bounces') }}</label>
            <span>
              <router-link :to="{ name: 'bounces', query: { campaign_id: props.row.id } }">
                {{ $utils.formatNumber(stats.bounces || 0) }}
              </router-link>
            </span>
          </p>
        </div>
      </b-table-column>

      <!-- Column 6: Actions -->
      <b-table-column v-slot="props" cell-class="actions" width="15%" align="right">
        <div>
          <template v-if="$can('campaigns:send')">
            <a v-if="props.row.status === 'paused' || props.row.status === 'draft'" href="#"
              @click.prevent="$utils.confirm('Activate sequence?', () => changeSequenceStatus(props.row, 'active'))"
              data-cy="btn-start" aria-label="Start Sequence">
              <b-tooltip label="Start / Resume Sequence" type="is-dark">
                <b-icon icon="rocket-launch-outline" size="is-small" />
              </b-tooltip>
            </a>

            <a v-if="props.row.status === 'active'" href="#"
              @click.prevent="$utils.confirm('Pause sequence?', () => changeSequenceStatus(props.row, 'paused'))"
              data-cy="btn-pause" aria-label="Pause Sequence">
              <b-tooltip label="Pause Sequence" type="is-dark">
                <b-icon icon="pause-circle-outline" size="is-small" />
              </b-tooltip>
            </a>
          </template>

          <a href="#" @click.prevent="previewSequence(props.row)" data-cy="btn-preview" aria-label="Preview">
            <b-tooltip label="Preview Content" type="is-dark">
              <b-icon icon="file-find-outline" size="is-small" />
            </b-tooltip>
          </a>

          <a v-if="$can('campaigns:manage')" href="#" @click.prevent="$utils.prompt($t('globals.buttons.clone'),
            {
              placeholder: $t('globals.fields.name'),
              value: 'Copy of ' + props.row.name,
            },
            (name) => cloneSequence(name, props.row))" data-cy="btn-clone" aria-label="Clone Sequence">
            <b-tooltip label="Clone Sequence" type="is-dark">
              <b-icon icon="file-multiple-outline" size="is-small" />
            </b-tooltip>
          </a>

          <router-link v-if="$can('campaigns:get_analytics')"
            :to="{ name: 'sequenceAnalytics', query: { id: props.row.id } }">
            <b-tooltip label="Sequence Analytics" type="is-dark">
              <b-icon icon="chart-bar" size="is-small" />
            </b-tooltip>
          </router-link>

          <a v-if="$can('campaigns:manage')" href="#"
            @click.prevent="deleteSequence(props.row)"
            data-cy="btn-delete" aria-label="Delete Sequence">
            <b-tooltip label="Delete" type="is-dark">
              <b-icon icon="trash-can-outline" size="is-small" />
            </b-tooltip>
          </a>
        </div>
      </b-table-column>

      <template #empty v-if="!loading">
        <empty-placeholder />
      </template>
    </b-table>

    <campaign-preview v-if="previewItem" type="campaign" :id="previewItem.id" :title="previewItem.name"
      @close="closePreview" />
  </section>
</template>

<script>
import CampaignPreview from '../components/CampaignPreview.vue';
import CopyText from '../components/CopyText.vue';
import EmptyPlaceholder from '../components/EmptyPlaceholder.vue';

export default {
  name: 'Sequences',
  components: {
    CampaignPreview,
    EmptyPlaceholder,
    CopyText,
  },
  data() {
    return {
      sequences: [],
      schedules: [],
      loading: false,
      searchQuery: '',
      perPage: 20,
      previewItem: null,
      queryParams: {
        page: 1,
        orderBy: 'created_at',
        order: 'desc',
      },
      pollID: null,
      sequenceStatsData: {},
      bulk: {
        checked: [],
        all: false,
      },
    };
  },
  computed: {
    filteredSequences() {
      if (!this.searchQuery.trim()) {
        return this.sequences;
      }
      const q = this.searchQuery.toLowerCase();
      return this.sequences.filter((s) => s.name && s.name.toLowerCase().includes(q));
    },
    numSelectedSequences() {
      return this.bulk.all ? this.filteredSequences.length : this.bulk.checked.length;
    },
  },
  mounted() {
    this.getSequences();
    this.getSchedules();
    this.pollStats();
  },
  destroyed() {
    clearInterval(this.pollID);
  },
  methods: {
    formatStatus(status) {
      if (!status || status === 'active') return 'Running';
      const key = `campaigns.status.${status}`;
      if (this.$te && this.$te(key)) {
        return this.$t(key);
      }
      return status.charAt(0).toUpperCase() + status.slice(1);
    },
    getSchedules() {
      this.$api.getSchedules().then((res) => {
        this.schedules = Array.isArray(res) ? res : (res.data || []);
      }).catch(() => {});
    },
    getScheduleName(id) {
      if (!id) return 'Default Schedule';
      const s = this.schedules.find((item) => item.id === id);
      return s ? s.name : `Schedule #${id}`;
    },
    getSequences() {
      this.loading = true;
      this.$api.getSequences().then((res) => {
        this.sequences = res.data || res || [];
        this.loading = false;
        this.bulk.checked = [];
      }).catch(() => {
        this.loading = false;
      });
    },
    isRunning(id) {
      return id in this.sequenceStatsData;
    },
    highlightedRow(data) {
      if (data.status === 'active') {
        return ['running'];
      }
      return '';
    },
    onSort(field, direction) {
      this.queryParams.orderBy = field;
      this.queryParams.order = direction;
    },
    getSequenceStats(seq) {
      if (seq.id in this.sequenceStatsData) {
        return this.sequenceStatsData[seq.id];
      }
      return seq;
    },
    pollStats() {
      clearInterval(this.pollID);
      this.pollID = setInterval(() => {
        this.$api.getSequenceAnalytics().then((data) => {
          if (data && data.stats) {
            this.sequenceStatsData = data.stats;
          }
        }).catch(() => {
          clearInterval(this.pollID);
        });
      }, 3000);
    },
    previewSequence(seq) {
      this.previewItem = seq;
    },
    closePreview() {
      this.previewItem = null;
    },
    changeSequenceStatus(seq, status) {
      this.$api.updateSequence(seq.id, {
        ...seq,
        status,
      }).then(() => {
        this.$utils.toast(`Sequence status updated to ${status}`);
        this.getSequences();
      });
    },
    cloneSequence(name, seq) {
      const cloned = {
        name,
        status: 'paused',
        schedule_id: seq.schedule_id,
        tags: seq.tags,
      };
      this.$api.createSequence(cloned).then((res) => {
        const newId = res.id || (res.data && res.data.id);
        this.$utils.toast('Sequence cloned successfully');
        if (newId) {
          this.$router.push({ name: 'sequence', params: { id: newId } });
        } else {
          this.getSequences();
        }
      });
    },
    onSelectAll() {
      this.bulk.all = true;
    },
    onTableCheck() {
      if (this.bulk.checked.length !== this.filteredSequences.length) {
        this.bulk.all = false;
      }
    },
    deleteSequence(seq) {
      this.$utils.confirm(`Delete sequence "${seq.name}"?`, () => {
        this.$api.deleteSequence(seq.id).then(() => {
          this.$utils.toast('Sequence deleted');
          this.getSequences();
        });
      });
    },
    deleteSelectedSequences() {
      const targets = this.bulk.all ? this.filteredSequences : this.bulk.checked;
      this.$utils.confirm(`Delete ${targets.length} selected sequence(s)?`, () => {
        const promises = targets.map((s) => this.$api.deleteSequence(s.id));
        Promise.all(promises).then(() => {
          this.$utils.toast(`${targets.length} sequence(s) deleted`);
          this.getSequences();
        });
      });
    },
  },
};
</script>
