import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type Email } from '@/lib/api'
import { parseCategoryValue } from '@/composables/useCategories'

// Optimistic star/read/category writes shared by the library rows and the viewer:
// the row updates at once and is put back (with a toast) if the server refuses.
// Both resolve to whether the write stuck, so a list can drop a row that no
// longer matches its filter.
export function useEmailActions() {
  const { t } = useI18n()

  async function toggleStar(e: Email): Promise<boolean> {
    const next = !e.starred
    e.starred = next
    try {
      await api.setStarred(e.sha256, next)
      return true
    } catch (err) {
      e.starred = !next
      toast.error(t('library.star_error'), { description: String(err) })
      return false
    }
  }

  async function toggleRead(e: Email): Promise<boolean> {
    const next = !e.read
    e.read = next
    try {
      await api.setRead(e.sha256, next)
      return true
    } catch (err) {
      e.read = !next
      toast.error(t('library.read_error'), { description: String(err) })
      return false
    }
  }

  async function setCategory(e: Email, value: string): Promise<boolean> {
    const next = parseCategoryValue(value)
    const prev = e.category_id
    e.category_id = next ?? undefined
    try {
      await api.setCategory(e.sha256, next)
      return true
    } catch (err) {
      e.category_id = prev
      toast.error(t('library.category_error'), { description: String(err) })
      return false
    }
  }

  return { toggleStar, toggleRead, setCategory }
}
