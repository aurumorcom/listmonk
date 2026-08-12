<template>
  <section class="analytics content relative">
    <h1 class="title is-4">
      Sequence Analytics
    </h1>
    <p class="has-text-grey">
      Performance metrics, funnel conversion, and time-series activity for cold outreach sequences.
    </p>

    <div v-if="serverConfig && serverConfig.privacy && (serverConfig.privacy.disable_tracking || !serverConfig.privacy.individual_tracking)"
      class="notification is-info">
      <template v-if="serverConfig.privacy.disable_tracking">
        {{ $t('analytics.trackingDisabled') }}
      </template>
      <template v-else-if="!serverConfig.privacy.individual_tracking">
        {{ $t('analytics.nonIndividualTracking') }}
      </template>
    </div>
    <hr />

    <!-- Search / Filter Form matching Campaign Analytics -->
    <form @submit.prevent="onSubmit">
      <div class="columns">
        <div class="column is-6">
          <b-field label="Sequences" label-position="on-border">
            <b-taginput v-model="form.sequences" :data="queriedSequences" name="sequences" ellipsis icon="tag-outline"
              placeholder="Select sequences..." autocomplete :allow-new="false" :open-on-focus="true"
              :before-adding="isSequenceSelected" @typing="querySequences" @focus="querySequences" field="name"
              :loading="isSearchLoading" />
          </b-field>
        </div>

        <div class="column is-5">
          <div class="columns">
            <div class="column is-6">
              <b-field data-cy="from" :label="$t('analytics.fromDate')" label-position="on-border">
                <b-datetimepicker v-model="form.from" icon="calendar-clock" :timepicker="{ hourFormat: '24' }"
                  :datetime-formatter="formatDateTime" @input="onFromDateChange" />
              </b-field>
            </div>
            <div class="column is-6">
              <b-field data-cy="to" :label="$t('analytics.toDate')" label-position="on-border">
                <b-datetimepicker v-model="form.to" icon="calendar-clock" :timepicker="{ hourFormat: '24' }"
                  :datetime-formatter="formatDateTime" @input="onToDateChange" />
              </b-field>
            </div>
          </div>
        </div>

        <div class="column is-1">
          <b-button native-type="submit" type="is-primary" icon-left="magnify" data-cy="btn-search" />
        </div>
      </div>
    </form>

    <!-- Sequence-Specific Top Metric Summary Cards -->
    <div class="columns mt-4">
      <div class="column is-3">
        <div class="box">
          <p class="heading">Active Contacts</p>
          <p class="title is-4">{{ $utils ? $utils.niceNumber(stats.activeContacts) : stats.activeContacts }}</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">Step Completions</p>
          <p class="title is-4">{{ $utils ? $utils.niceNumber(stats.stepCompletions) : stats.stepCompletions }}</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">Reply Rate</p>
          <p class="title is-4">{{ stats.replyRate }}%</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">Conversion Rate</p>
          <p class="title is-4">{{ stats.conversionRate }}%</p>
        </div>
      </div>
    </div>

    <!-- Step Conversion Funnel Section -->
    <section class="charts mt-4">
      <div class="box">
        <h4 class="subtitle is-5">Step Conversion Funnel</h4>
        <chart type="bar" v-if="!loading" :data="funnelChartData" />
      </div>
    </section>

    <!-- Time Series Activity Charts (Views, Clicks, Bounces, Links) from Campaign Analytics -->
    <section class="charts mt-5">
      <div class="chart" v-for="(v, k) in timeCharts" :key="k">
        <div class="columns">
          <div class="column is-9">
            <b-loading v-if="v.loading" :active="v.loading" :is-full-page="false" />
            <h4>
              {{ v.name }}
              <span v-if="v.type !== 'bar' && counts[k] !== undefined" class="has-text-grey-light">({{ $utils ? $utils.niceNumber(counts[k]) : counts[k] }})</span>
            </h4>
            <chart :type="v.type" v-if="!v.loading" :data="v.data" :on-click="v.onClick" />
          </div>
          <div class="column is-2 donut-container" v-if="v.donutData">
            <chart type="donut" v-if="!v.loading" :data="v.donutData" />
          </div>
        </div>
      </div>
    </section>
  </section>
</template>

<script>
import dayjs from 'dayjs';
import Vue from 'vue';
import { mapState } from 'vuex';
import Chart from '../components/Chart.vue';
import { colors } from '../constants';

const chartColorRed = '#ee7d5b';
const chartColors = [
  colors ? colors.primary : '#22c55e',
  '#FFB50D',
  '#41AC9C',
  chartColorRed,
  '#7FC7BC',
  '#3a82d6',
  '#688ED9',
  '#FFC43D',
];

export default Vue.extend({
  name: 'SequenceAnalytics',
  components: {
    Chart,
  },
  data() {
    return {
      loading: true,
      isSearchLoading: false,
      queriedSequences: [],
      stats: {
        activeContacts: 0,
        stepCompletions: 0,
        replyRate: '0.0',
        conversionRate: '0.0',
      },
      counts: {
        views: 0,
        clicks: 0,
        bounces: 0,
        links: 0,
      },
      urls: [],
      funnelChartData: {
        labels: [],
        datasets: [
          { name: 'Contacts Reached', values: [] },
          { name: 'Replies', values: [] },
        ],
      },
      timeCharts: {
        views: {
          name: 'Views',
          type: 'line',
          data: null,
          fn: this.$api.getCampaignViewCounts,
          chartFn: this.makeLineCharts,
          loading: false,
        },
        clicks: {
          name: 'Clicks',
          type: 'line',
          data: null,
          fn: this.$api.getCampaignClickCounts,
          chartFn: this.makeLineCharts,
          loading: false,
        },
        bounces: {
          name: 'Bounces',
          type: 'line',
          data: null,
          fn: this.$api.getCampaignBounceCounts,
          chartFn: this.makeLineCharts,
          loading: false,
        },
        links: {
          name: 'Links Clicked',
          type: 'bar',
          data: null,
          loading: false,
          fn: this.$api.getCampaignLinkCounts,
          chartFn: this.makeLinksChart,
          onClick: this.onLinkClick,
        },
      },
      form: {
        sequences: [],
        from: null,
        to: null,
      },
    };
  },
  computed: {
    ...mapState(['serverConfig']),
  },
  created() {
    const now = dayjs().set('hour', 23).set('minute', 59).set('seconds', 0);
    const weekAgo = now.subtract(7, 'day').set('hour', 0).set('minute', 0);
    const from = this.$route.query.from ? dayjs.unix(this.$route.query.from) : weekAgo;
    const to = this.$route.query.to ? dayjs.unix(this.$route.query.to) : now;
    this.form.from = from.toDate();
    this.form.to = to.toDate();
  },
  mounted() {
    this.fetchSequenceKPIs();

    const ids = this.$utils ? this.$utils.parseQueryIDs(this.$route.query.id) : [];
    if (ids.length > 0) {
      this.isSearchLoading = true;
      Promise.allSettled(ids.map((id) => this.$api.getSequence(id))).then((res) => {
        res.forEach((d) => {
          if (d.status === 'fulfilled' && d.value) {
            const seq = d.value.data || d.value;
            seq.name = `#${seq.id}: ${seq.name}`;
            this.form.sequences.push(seq);
          }
        });
        this.isSearchLoading = false;
        this.loadTimeSeriesCharts();
      });
    } else {
      this.loadTimeSeriesCharts();
    }
  },
  methods: {
    onFromDateChange() {
      if (this.form.from > this.form.to) {
        this.form.to = dayjs(this.form.from).add(7, 'day').toDate();
      }
    },
    onToDateChange() {
      if (this.form.from > this.form.to) {
        this.form.from = dayjs(this.form.to).add(-7, 'day').toDate();
      }
    },
    formatDateTime(s) {
      return dayjs(s).format('YYYY-MM-DD HH:mm');
    },
    isSequenceSelected(seq) {
      return !this.form.sequences.find(({ id }) => id === seq.id);
    },
    querySequences(q) {
      this.isSearchLoading = true;
      this.$api.getSequences().then((res) => {
        this.isSearchLoading = false;
        const list = res.data || res || [];
        this.queriedSequences = list
          .filter((s) => !q || (s.name && s.name.toLowerCase().includes(q.toLowerCase())))
          .map((s) => ({ ...s, name: `#${s.id}: ${s.name}` }));
      }).catch(() => {
        this.isSearchLoading = false;
      });
    },
    onSubmit() {
      this.$router.push({
        query: {
          id: this.form.sequences.map((s) => s.id),
          from: dayjs(this.form.from).unix(),
          to: dayjs(this.form.to).unix(),
        },
      });
      this.fetchSequenceKPIs();
      this.loadTimeSeriesCharts();
    },
    fetchSequenceKPIs() {
      this.loading = true;
      this.$api.getSequenceAnalytics().then((res) => {
        const d = res.data || res || {};
        this.stats = {
          activeContacts: d.active_contacts || 0,
          stepCompletions: d.step_completions || 0,
          replyRate: (d.reply_rate || 0).toFixed(1),
          conversionRate: (d.conversion_rate || 0).toFixed(1),
        };

        const funnel = d.funnel || [];
        if (funnel.length > 0) {
          const labels = [];
          const reachedVals = [];
          const repliedVals = [];
          funnel.forEach((f) => {
            labels.push(`Step ${f.step_number}${f.subject ? `: ${f.subject}` : ''}`);
            reachedVals.push(f.reached || 0);
            repliedVals.push(f.replied || 0);
          });
          this.funnelChartData = {
            labels,
            datasets: [
              { name: 'Contacts Reached', values: reachedVals },
              { name: 'Replies', values: repliedVals },
            ],
          };
        } else {
          this.funnelChartData = {
            labels: ['No Sequence Steps Configured'],
            datasets: [
              { name: 'Contacts Reached', values: [0] },
              { name: 'Replies', values: [0] },
            ],
          };
        }
        this.loading = false;
      }).catch(() => {
        this.loading = false;
      });
    },
    loadTimeSeriesCharts() {
      const selectedIDs = this.form.sequences.map((s) => s.id);
      Object.keys(this.timeCharts).forEach((k) => {
        this.timeCharts[k].data = null;
        this.timeCharts[k].donutData = null;
        this.fetchTimeSeriesData(k, selectedIDs);
      });
    },
    fetchTimeSeriesData(typ, ids) {
      if (!this.timeCharts[typ].fn) return;
      this.timeCharts[typ].loading = true;
      this.timeCharts[typ].fn({
        id: ids,
        from: this.form.from,
        to: this.form.to,
      }).then((data) => {
        const list = data || [];
        this.counts[typ] = list.reduce((sum, d) => sum + (d.count || 0), 0);
        const { points, donut } = this.timeCharts[typ].chartFn(typ, this.form.sequences, list);
        this.timeCharts[typ].data = points;
        this.timeCharts[typ].donutData = donut;
        this.timeCharts[typ].loading = false;
      }).catch(() => {
        this.timeCharts[typ].loading = false;
      });
    },
    makeLineCharts(typ, sequences, data) {
      const seqMap = sequences.reduce((obj, s) => ({ ...obj, [s.id]: s }), {});
      const seqIDs = Object.keys(seqMap);
      const lines = seqIDs.map((id, n) => {
        const sId = parseInt(id, 10);
        const points = data.filter((item) => item.campaignId === sId || item.sequenceId === sId);
        return {
          label: seqMap[id].name,
          data: points.map((item) => ({ x: this.formatDateTime(item.timestamp), y: item.count })),
          borderColor: chartColors[n % chartColors.length],
          borderWidth: 2,
        };
      });

      const labels = [];
      const points = seqIDs.map((id) => {
        labels.push(seqMap[id].name);
        const sId = parseInt(id, 10);
        return data.reduce((a, item) => (item.campaignId === sId || item.sequenceId === sId ? a + item.count : a), 0);
      });

      const donut = {
        labels,
        datasets: [{ data: points, backgroundColor: chartColors, borderWidth: 6 }],
      };
      return { points: { datasets: lines }, donut };
    },
    makeLinksChart(typ, sequences, data) {
      const labels = (data || []).map((l) => {
        try {
          this.urls.push(l.url);
          const u = new URL(l.url);
          return u.hostname + u.pathname;
        } catch {
          return l.url;
        }
      });
      const out = {
        labels,
        datasets: [
          { data: (data || []).map((l) => l.count), backgroundColor: chartColors },
        ],
      };
      return { points: out, donut: null };
    },
    onLinkClick(e) {
      if (!e || !e.chart) return;
      const bars = e.chart.getElementsAtEventForMode(e, 'nearest', { intersect: true }, true);
      if (bars.length > 0 && this.urls[bars[0].index]) {
        window.open(this.urls[bars[0].index], '_blank', 'noopener noreferrer');
      }
    },
  },
});
</script>
