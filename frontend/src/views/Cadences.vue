<template>
  <section class="cadences">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">
          Cadences
        </h1>
      </div>
      <div class="column has-text-right">
        <b-button :to="{ name: 'cadence', params: { id: 'new' } }" tag="router-link" type="is-primary" icon-left="plus">
          New Cadence
        </b-button>
      </div>
    </header>

    <b-table :data="cadences" :loading="loading" hoverable>
      <b-table-column v-slot="props" field="name" label="Name">
        <router-link :to="{ name: 'cadence', params: { id: props.row.id } }">
          <strong>{{ props.row.name }}</strong>
        </router-link>
      </b-table-column>

      <b-table-column v-slot="props" field="status" label="Status">
        <span :class="['tag', props.row.status === 'active' ? 'is-success' : 'is-light']">
          {{ props.row.status }}
        </span>
      </b-table-column>

      <b-table-column v-slot="props" field="created_at" label="Created">
        {{ new Date(props.row.created_at).toLocaleDateString() }}
      </b-table-column>

      <b-table-column v-slot="props" label="Actions">
        <b-button size="is-small" type="is-danger" icon-left="trash-can-outline" @click="deleteCadence(props.row.id)" />
      </b-table-column>
    </b-table>
  </section>
</template>

<script>
export default {
  name: 'Cadences',
  data() {
    return {
      cadences: [],
      loading: false,
    };
  },
  mounted() {
    this.getCadences();
  },
  methods: {
    getCadences() {
      this.loading = true;
      this.$api.getCadences().then((res) => {
        this.cadences = res.data;
        this.loading = false;
      }).catch(() => {
        this.loading = false;
      });
    },
    deleteCadence(id) {
      if (confirm('Are you sure you want to delete this cadence?')) {
        this.$api.deleteCadence(id).then(() => {
          this.getCadences();
        });
      }
    },
  },
};
</script>
