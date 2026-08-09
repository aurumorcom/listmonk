<template>
  <div class="content">
    <div class="columns is-mobile my-0">
      <div class="column">
        <h2>{{ $t('menu.allContacts') }}</h2>
      </div>
      <div class="column has-text-right">
        <b-button v-if="$can('subscribers:manage')" type="is-primary" icon-left="plus" @click="showNewForm">
          New Contact
        </b-button>
      </div>
    </div>

    <form @submit.prevent="onSubmit">
      <b-field>
        <b-input
          v-model="queryParams.query"
          placeholder="Search by name, email, or phone..."
          type="search"
          icon="magnify"
          expanded
        />
        <p class="control">
          <b-button class="is-primary" native-type="submit">
            Search
          </b-button>
        </p>
      </b-field>
    </form>

    <b-table
      :data="subscribers.results || []"
      :loading="loading.subscribers"
      paginated
      backend-pagination
      :total="subscribers.total || 0"
      :per-page="subscribers.perPage || 50"
      @page-change="onPageChange"
      hoverable
    >
      <b-table-column v-slot="props" field="name" label="Name">
        <a href="#" @click.prevent="editContact(props.row)">
          <strong>{{ props.row.name || '-' }}</strong>
        </a>
      </b-table-column>

      <b-table-column v-slot="props" field="email" label="Email">
        {{ props.row.email }}
      </b-table-column>

      <b-table-column v-slot="props" field="phone" label="Phone">
        {{ props.row.phone || '-' }}
      </b-table-column>

      <b-table-column v-slot="props" field="status" label="Status">
        <b-tag :class="props.row.status">
          {{ props.row.status }}
        </b-tag>
      </b-table-column>

      <b-table-column v-slot="props" label="Actions" numeric>
        <b-button size="is-small" type="is-light" icon-left="pencil" @click="editContact(props.row)" />
        <b-button size="is-small" type="is-danger" icon-left="delete" @click="deleteContact(props.row)" />
      </b-table-column>

      <template #empty>
        <div class="has-text-centered my-6">
          <p class="is-size-5">No contacts found.</p>
        </div>
      </template>
    </b-table>

    <b-modal scroll="keep" :active.sync="isFormVisible" :width="600">
      <contact-form :data="curItem" :is-editing="isEditing" @finished="fetchContacts" />
    </b-modal>
  </div>
</template>

<script>
import { mapState } from 'vuex';
import ContactForm from './ContactForm.vue';

export default {
  name: 'Contacts',
  components: {
    ContactForm,
  },
  data() {
    return {
      queryParams: {
        query: '',
        page: 1,
      },
      isFormVisible: false,
      isEditing: false,
      curItem: {},
    };
  },
  computed: {
    ...mapState(['subscribers', 'loading']),
  },
  methods: {
    fetchContacts() {
      this.$api.getContacts(this.queryParams);
    },
    onSubmit() {
      this.queryParams.page = 1;
      this.fetchContacts();
    },
    onPageChange(p) {
      this.queryParams.page = p;
      this.fetchContacts();
    },
    showNewForm() {
      this.curItem = {};
      this.isEditing = false;
      this.isFormVisible = true;
    },
    editContact(row) {
      this.curItem = row;
      this.isEditing = true;
      this.isFormVisible = true;
    },
    deleteContact(row) {
      this.$utils.confirm(`Delete contact ${row.name || row.email}?`, () => {
        this.$api.deleteContacts({ id: [row.id] }).then(() => {
          this.$utils.toast('Contact deleted');
          this.fetchContacts();
        });
      });
    },
  },
  mounted() {
    this.fetchContacts();
  },
};
</script>
