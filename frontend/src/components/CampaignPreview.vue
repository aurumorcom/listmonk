<template>
  <div>
    <b-modal scroll="keep" @close="close" :aria-modal="true" :active="isVisible">
      <div>
        <div class="modal-card" style="width: auto">
          <header class="modal-card-head" style="justify-content: space-between; align-items: center;">
            <h4>{{ title }}</h4>
            <div style="display: flex; align-items: center; gap: 10px;">
              <b-field label="Select contact" label-position="on-border" style="margin-bottom:0; min-width: 220px;">
                <b-autocomplete
                  v-model="contactSearch"
                  :data="filteredContacts"
                  placeholder="Search name, email, phone..."
                  field="displayLabel"
                  :loading="isSearchingContacts"
                  size="is-small"
                  open-on-focus
                  @typing="getAsyncContacts"
                  @select="onSelectContact">
                  <template #empty>No contacts found</template>
                </b-autocomplete>
              </b-field>
              <b-button v-if="id && type === 'template'" size="is-small" type="is-info" icon-left="email-send-outline" @click="sendTest">
                Send Test
              </b-button>
            </div>
          </header>
        </div>
        <section expanded class="modal-card-body preview">
          <b-loading :active="isLoading" :is-full-page="false" />
          <form v-if="isPost" method="post" :action="previewURL" target="iframe" ref="form">
            <input v-if="templateId" type="hidden" name="template_id" :value="templateId" />
            <input v-if="contentType" type="hidden" name="content_type" :value="contentType" />
            <input v-if="templateType" type="hidden" name="template_type" :value="templateType" />
            <input v-if="archiveMeta" type="hidden" name="archive_meta" :value="archiveMeta" />
            <input v-if="body" type="hidden" name="body" :value="body" />
            <input v-if="selectedSubscriberId" type="hidden" name="subscriber_id" :value="selectedSubscriberId" />
          </form>

          <iframe id="iframe" name="iframe" ref="iframe" :title="title" :src="isPost ? 'about:blank' : previewURL"
            @load="onLoaded" sandbox="allow-scripts" />
        </section>
        <footer class="modal-card-foot has-text-right">
          <b-button @click="close">
            {{ $t('globals.buttons.close') }}
          </b-button>
        </footer>
      </div>
    </b-modal>
  </div>
</template>

<script>
import { uris } from '../constants';

export default {
  name: 'CampaignPreview',

  props: {
    isPost: { type: Boolean, default: false },

    // Template or campaign ID.
    id: { type: Number, default: 0 },
    title: { type: String, default: '' },

    // campaign | template.
    type: { type: String, default: '' },

    // campaign | tx.
    templateType: { type: String, default: '' },

    archiveMeta: { type: String, default: null },

    body: { type: String, default: '' },
    contentType: { type: String, default: '' },
    templateId: { type: [Number, null], default: null },
    isArchive: { type: Boolean, default: false },
  },

  data() {
    return {
      isVisible: true,
      isLoading: true,
      formSubmitted: false,
      selectedSubscriberId: '',
      contactSearch: '',
      filteredContacts: [],
      isSearchingContacts: false,
      searchDebounceTimer: null,
    };
  },

  methods: {
    close() {
      this.$emit('close');
      this.isVisible = false;
    },

    getAsyncContacts(query) {
      if (this.searchDebounceTimer) {
        clearTimeout(this.searchDebounceTimer);
      }
      this.searchDebounceTimer = setTimeout(() => {
        this.isSearchingContacts = true;
        this.$api.getSubscribers({ search: query, per_page: 20 }).then((res) => {
          this.isSearchingContacts = false;
          const results = res.results || (res.data ? res.data.results : []) || [];
          this.filteredContacts = results.map((c) => ({
            ...c,
            displayLabel: c.name ? `${c.name} (${c.email || c.phone || c.id})` : `${c.email || c.phone || `ID: ${c.id}`}`,
          }));
        }).catch(() => {
          this.isSearchingContacts = false;
          this.filteredContacts = [];
        });
      }, 300);
    },

    onSelectContact(option) {
      if (option && option.id) {
        this.selectedSubscriberId = option.id;
        this.reloadPreview();
      } else {
        this.selectedSubscriberId = '';
        this.reloadPreview();
      }
    },

    // On iframe load, kill the spinner.
    onLoaded() {
      if (!this.isPost) {
        this.isLoading = false;
        return;
      }

      if (this.formSubmitted) {
        this.isLoading = false;
      }
    },

    reloadPreview() {
      if (this.isPost && this.$refs.form) {
        this.isLoading = true;
        this.$refs.form.submit();
      }
    },

    async sendTest() {
      // eslint-disable-next-line no-alert
      const email = window.prompt('Enter email address to send test message:');
      if (!email) return;
      try {
        await this.$api.post(`/api/templates/${this.id}/test`, {
          subscriber_id: parseInt(this.selectedSubscriberId, 10) || 0,
          test_email: email,
        });
        this.$buefy.toast.open({ message: 'Test message dispatched successfully', type: 'is-success' });
      } catch (e) {
        this.$buefy.toast.open({ message: 'Failed to send test message', type: 'is-danger' });
      }
    },
  },

  computed: {
    previewURL() {
      let uri = 'about:blank';

      if (this.type === 'campaign') {
        uri = this.isArchive ? uris.previewCampaignArchive : uris.previewCampaign;
      } else if (this.type === 'template') {
        if (this.id) {
          uri = uris.previewTemplate;
        } else {
          uri = uris.previewRawTemplate;
        }
      }

      return uri.replace(':id', this.id);
    },
  },

  mounted() {
    if (this.isPost) {
      setTimeout(() => {
        this.$refs.form.submit();
        this.formSubmitted = true;
      }, 100);
    }
  },
};
</script>
