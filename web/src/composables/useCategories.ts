import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, type Category } from '@/lib/api'

// Module scope, like useHealth: the library rows, the viewer, the filter and the
// settings page all read the same list, and a rename has to repaint every dot at
// once. Emails carry only category_id, so this is what resolves it to a colour.
const categories = ref<Category[]>([])
const loaded = ref(false)
let inFlight: Promise<void> | null = null

const byId = computed(() => new Map(categories.value.map((c) => [c.id, c])))

async function load(force = false): Promise<void> {
  if (loaded.value && !force) return
  // Several components mount together on first paint; one request serves them.
  if (!inFlight) {
    inFlight = api
      .listCategories()
      .then((list) => {
        categories.value = list
        loaded.value = true
      })
      .finally(() => {
        inFlight = null
      })
  }
  return inFlight
}

export function useCategories() {
  const { t } = useI18n()

  // An unnamed category shows its colour's name in the current language. The
  // server seeds names empty precisely so this stays translatable.
  const labelFor = (c: Category | undefined) =>
    c ? c.name || t(`settings.color.${c.color}`) : ''

  return {
    categories,
    byId,
    labelFor,
    load,
    reload: () => load(true),
  }
}
