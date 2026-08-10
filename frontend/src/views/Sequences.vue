<template>
  <section class="sequences">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">
          Sequences
        </h1>
      </div>
      <div class="column has-text-right">
        <b-button :to="{ name: 'sequence', params: { id: 'new' } }" tag="router-link" type="is-primary" icon-left="plus">
          New Sequence
        </b-button>
      </div>
    </header>

    <b-table :data="sequences" :loading="loading" hoverable>
      <b-table-column v-slot="props" field="name" label="Name">
        <router-link :to="{ name: 'sequence', params: { id: props.row.id } }">
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
        <b-button size="is-small" type="is-danger" icon-left="trash-can-outline" @click="deleteSequence(props.row.id)" />
      </b-table-column>
    </b-table>
  </section>
</template>

<script>
export default {
  name: 'Sequences',
  data() {
    return {
      sequences: [],
      loading: false,
    };
  },
  mounted() {
    this.getSequences();
  },
  methods: {
    getSequences() {
      this.loading = true;
      this.$api.getSequences().then((res) => {
        this.sequences = res.data;
        this.loading = false;
      }).catch(() => {
        this.loading = false;
      });
    },
    deleteSequence(id) {
      if (confirm('Are you sure you want to delete this sequence?')) {
        this.$api.deleteSequence(id).then(() => {
          this.getSequences();
        });
      }
    },
  },
};
</script>
