<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Eye, EyeOff, ShieldAlert } from 'lucide-vue-next'
import { api, type MCPInfo } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import CopyBlock from '@/components/CopyBlock.vue'

// Lets an AI agent (Claude Desktop, Claude Code, any MCP client) search and
// read the archive. Off by default; turning it on says plainly that what an
// agent reads goes to that agent's AI provider.

const { t } = useI18n()
const info = ref<MCPInfo | null>(null)
const busy = ref(false)
const confirmOn = ref(false)
const confirmRotate = ref(false)
const reveal = ref(false)

type Client = 'claude-desktop' | 'claude-code' | 'other'
const client = ref<Client>('claude-desktop')
const clients: Client[] = ['claude-desktop', 'claude-code', 'other']

onMounted(load)

async function load() {
  try {
    info.value = await api.getMCP()
  } catch (e) {
    toast.error(t('agents.load_error'), { description: String(e) })
  }
}

// The switch only reflects the saved state; it moves after the server agrees.
const enabled = computed({
  get: () => info.value?.enabled ?? false,
  set: (on: boolean) => {
    if (on) confirmOn.value = true
    else void apply(false)
  },
})

async function apply(on: boolean) {
  busy.value = true
  try {
    info.value = await api.setMCP(on)
    toast.success(t(on ? 'agents.on' : 'agents.off'))
  } catch (e) {
    toast.error(t('agents.error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}

async function rotate() {
  busy.value = true
  try {
    info.value = await api.rotateMCPToken()
    toast.success(t('agents.rotated'))
  } catch (e) {
    toast.error(t('agents.error'), { description: String(e) })
  } finally {
    busy.value = false
  }
}

const token = computed(() => info.value?.token ?? '')
// Enough of the token to tell two apart, never enough to use.
const masked = computed(() => (token.value ? token.value.slice(0, 13) + '•'.repeat(12) : ''))
const shownToken = computed(() => (reveal.value ? token.value : masked.value))

const desktopConfig = computed(() => {
  const i = info.value
  if (!i) return ''
  // Desktop agents start the bridge themselves; it reads the token from the
  // data folder, so this config holds no secret. Under WSL, a Windows app
  // reaches the Linux binary through wsl.exe.
  const server = i.wsl
    ? { command: 'wsl.exe', args: ['-e', i.executable, 'mcp'] }
    : { command: i.executable, args: ['mcp'] }
  return JSON.stringify({ mcpServers: { 'local-eml': server } }, null, 2)
})

function claudeCodeCommand(tok: string) {
  return `claude mcp add --transport http local-eml ${info.value?.url ?? ''} --header "Authorization: Bearer ${tok}"`
}

function headerLine(tok: string) {
  return `Authorization: Bearer ${tok}`
}
</script>

<template>
  <div class="space-y-6">
    <Card class="p-6 space-y-4">
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1">
          <h2 class="text-lg font-semibold">{{ t('agents.title') }}</h2>
          <p class="text-sm text-muted-foreground break-keep">{{ t('agents.help') }}</p>
        </div>
        <Switch
          v-model="enabled"
          :disabled="!info || busy"
          :aria-label="t('agents.toggle')"
          class="mt-1"
        />
      </div>

      <div
        role="note"
        class="flex gap-2.5 rounded-sm bg-amber-500/15 p-3 text-sm text-amber-900 dark:text-amber-200 break-keep"
      >
        <ShieldAlert class="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
        <div class="space-y-1">
          <p class="font-medium">{{ t('agents.warning_title') }}</p>
          <p>{{ t('agents.warning_body') }}</p>
        </div>
      </div>

      <ul class="text-sm text-muted-foreground space-y-1 list-disc pl-5 break-keep">
        <li>{{ t('agents.point_readonly') }}</li>
        <li>{{ t('agents.point_local') }}</li>
        <li>{{ t('agents.point_flagged') }}</li>
      </ul>
    </Card>

    <template v-if="info?.enabled">
      <Card class="p-6 space-y-4">
        <h2 class="text-lg font-semibold">{{ t('agents.setup_title') }}</h2>

        <div role="tablist" class="inline-flex rounded-sm border border-hairline p-0.5 text-sm">
          <button
            v-for="c in clients"
            :key="c"
            type="button"
            role="tab"
            :aria-selected="client === c"
            :class="[
              'px-3 h-7 rounded-[3px] transition-colors',
              client === c ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:text-foreground',
            ]"
            @click="client = c"
          >
            {{ t(`agents.client_${c.replace('-', '_')}`) }}
          </button>
        </div>

        <div v-if="client === 'claude-desktop'" class="space-y-3 text-sm break-keep">
          <p>{{ t('agents.desktop_step') }}</p>
          <CopyBlock :text="desktopConfig" :label="t('agents.client_claude_desktop')" />
          <p class="text-xs text-muted-foreground">{{ t('agents.desktop_note') }}</p>
          <p v-if="info.wsl" class="text-xs text-muted-foreground">{{ t('agents.desktop_wsl') }}</p>
        </div>

        <div v-else-if="client === 'claude-code'" class="space-y-3 text-sm break-keep">
          <p>{{ t('agents.code_step') }}</p>
          <CopyBlock
            :text="claudeCodeCommand(token)"
            :shown="claudeCodeCommand(shownToken)"
            :label="t('agents.client_claude_code')"
          />
          <p class="text-xs text-muted-foreground">{{ t('agents.port_note') }}</p>
        </div>

        <div v-else class="space-y-3 text-sm break-keep">
          <p>{{ t('agents.other_http') }}</p>
          <CopyBlock :text="info.url" :label="t('agents.url')" />
          <CopyBlock :text="headerLine(token)" :shown="headerLine(shownToken)" :label="t('agents.token')" />
          <p>{{ t('agents.other_stdio') }}</p>
          <CopyBlock :text="(info.wsl ? `wsl.exe -e ${info.executable}` : info.executable) + ' mcp'" />
          <p class="text-xs text-muted-foreground">{{ t('agents.port_note') }}</p>
        </div>
      </Card>

      <Card class="p-6 space-y-3">
        <h2 class="text-lg font-semibold">{{ t('agents.token_title') }}</h2>
        <p class="text-sm text-muted-foreground break-keep">{{ t('agents.token_help') }}</p>
        <div class="flex flex-wrap items-center gap-2">
          <code class="flex-1 min-w-0 truncate rounded-sm bg-muted px-3 py-2 font-mono text-xs" data-testid="mcp-token">
            {{ shownToken }}
          </code>
          <Button variant="outline" size="sm" :aria-pressed="reveal" @click="reveal = !reveal">
            <EyeOff v-if="reveal" class="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
            <Eye v-else class="h-3.5 w-3.5 mr-1.5" aria-hidden="true" />
            {{ reveal ? t('agents.hide') : t('agents.show') }}
          </Button>
          <Button variant="outline" size="sm" :disabled="busy" @click="confirmRotate = true">
            {{ t('agents.rotate') }}
          </Button>
        </div>
      </Card>

      <Card class="p-6 space-y-3">
        <h2 class="text-lg font-semibold">{{ t('agents.tools_title') }}</h2>
        <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
          <template v-for="name in info.tools" :key="name">
            <dt class="font-mono text-xs pt-0.5">{{ name }}</dt>
            <dd class="text-muted-foreground break-keep">{{ t(`agents.tool_${name}`) }}</dd>
          </template>
        </dl>
      </Card>
    </template>

    <ConfirmDialog
      v-model:open="confirmOn"
      :title="t('agents.confirm_title')"
      :description="t('agents.confirm_body')"
      :confirm-label="t('agents.confirm_on')"
      :cancel-label="t('agents.cancel')"
      @confirm="apply(true)"
    />
    <ConfirmDialog
      v-model:open="confirmRotate"
      :title="t('agents.rotate_title')"
      :description="t('agents.rotate_body')"
      :confirm-label="t('agents.rotate')"
      :cancel-label="t('agents.cancel')"
      destructive
      @confirm="rotate"
    />
  </div>
</template>
