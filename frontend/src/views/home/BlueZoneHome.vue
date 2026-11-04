<template>
  <div class="blue-home" :class="{ 'theme-dark': isDark }" data-testid="blue-zone-home">
    <a class="blue-skip-link" href="#blue-main">{{ t('home.blueZone.skipToContent') }}</a>

    <header class="blue-header">
      <div class="blue-container blue-nav-shell">
        <a class="blue-brand" href="#blue-top" :aria-label="t('home.blueZone.homeAriaLabel')">
          <span class="blue-brand-mark" aria-hidden="true"></span>
          {{ t('home.blueZone.brand') }}
        </a>

        <nav class="blue-desktop-nav" :aria-label="t('home.blueZone.primaryNavigation')">
          <a href="#blue-advantages">{{ t('home.blueZone.nav.advantages') }}</a>
          <RouterLink v-if="showModelPlazaEntry" to="/model-plaza">
            {{ t('home.blueZone.nav.models') }}
          </RouterLink>
          <a v-else href="#blue-models">{{ t('home.blueZone.nav.models') }}</a>
          <a href="#blue-agents">{{ t('home.blueZone.nav.agents') }}</a>
          <a href="#blue-scale">{{ t('home.blueZone.nav.scale') }}</a>
        </nav>

        <div class="blue-nav-actions">
          <LocaleSwitcher />
          <button
            type="button"
            class="blue-icon-button"
            :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="emit('toggleTheme')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <RouterLink class="blue-button blue-header-button" :to="primaryDestination">
            {{ primaryLabel }}
          </RouterLink>
          <button
            type="button"
            class="blue-menu-button"
            :aria-label="menuOpen ? t('home.blueZone.closeMenu') : t('home.blueZone.openMenu')"
            :aria-expanded="menuOpen"
            aria-controls="blue-mobile-panel"
            @click="menuOpen = !menuOpen"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path v-if="!menuOpen" d="M4 7h16M4 12h16M4 17h16" />
              <path v-else d="m6 6 12 12M18 6 6 18" />
            </svg>
          </button>
        </div>
      </div>

      <nav
        id="blue-mobile-panel"
        class="blue-mobile-panel"
        :class="{ open: menuOpen }"
        :aria-label="t('home.blueZone.mobileNavigation')"
      >
        <a href="#blue-advantages" @click="closeMenu">{{ t('home.blueZone.nav.advantages') }}</a>
        <RouterLink v-if="showModelPlazaEntry" to="/model-plaza" @click="closeMenu">
          {{ t('home.blueZone.nav.models') }}
        </RouterLink>
        <a v-else href="#blue-models" @click="closeMenu">{{ t('home.blueZone.nav.models') }}</a>
        <a href="#blue-agents" @click="closeMenu">{{ t('home.blueZone.nav.agents') }}</a>
        <a href="#blue-scale" @click="closeMenu">{{ t('home.blueZone.nav.scale') }}</a>
        <RouterLink class="blue-button" :to="primaryDestination" @click="closeMenu">
          {{ primaryLabel }}
        </RouterLink>
      </nav>
    </header>

    <main id="blue-main">
      <section id="blue-top" class="blue-hero" :aria-labelledby="heroTitleId">
        <div class="blue-container blue-hero-grid">
          <div class="blue-hero-copy">
            <h1 :id="heroTitleId">
              {{ t('home.blueZone.hero.lineOne') }}<br />{{ t('home.blueZone.hero.lineTwo') }}
            </h1>
            <div class="blue-hero-actions">
              <RouterLink class="blue-button" :to="primaryDestination">{{ primaryLabel }}</RouterLink>
              <RouterLink
                v-if="!isAuthenticated && registrationEnabled"
                class="blue-button blue-button-secondary"
                to="/register"
              >
                {{ t('home.blueZone.register') }}
              </RouterLink>
            </div>
            <p class="blue-trust-line" :aria-label="t('home.blueZone.hero.trustAriaLabel')">
              <span>{{ t('home.blueZone.hero.trust.unified') }}</span>
              <span>{{ t('home.blueZone.hero.trust.choose') }}</span>
              <span>{{ t('home.blueZone.hero.trust.transparent') }}</span>
            </p>
          </div>

          <article
            ref="assistantCardRef"
            class="blue-assistant-card"
            :class="{ switching: isSwitching }"
            :aria-labelledby="assistantTitleId"
          >
            <div class="blue-assistant-head">
              <h2 :id="assistantTitleId" class="blue-assistant-title">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                  <path d="m12 3 8 4-8 4-8-4 8-4Z" />
                  <path d="m4 12 8 4 8-4M4 17l8 4 8-4" />
                </svg>
                {{ t('home.blueZone.demo.title') }}
              </h2>
              <div class="blue-status-list" :aria-label="t('home.blueZone.demo.statusAriaLabel')">
                <span class="blue-status-chip"><span class="blue-dot" aria-hidden="true"></span>{{ t('home.blueZone.demo.smartRouting') }}</span>
                <span class="blue-status-chip success"><span class="blue-dot" aria-hidden="true"></span>{{ t('home.blueZone.demo.connected') }}</span>
              </div>
            </div>

            <div class="blue-conversation-row">
              <div class="blue-avatar" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                  <circle cx="12" cy="8" r="3" />
                  <path d="M5 20c.5-4 2.8-6 7-6s6.5 2 7 6" />
                </svg>
              </div>
              <div class="blue-prompt-bubble">{{ t(currentDemo.promptKey) }}</div>
            </div>

            <div class="blue-conversation-row">
              <div class="blue-avatar" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                  <path d="m12 3 8 4-8 4-8-4 8-4Z" />
                  <path d="m4 12 8 4 8-4M4 17l8 4 8-4" />
                </svg>
              </div>
              <div class="blue-code-shell" :aria-label="t('home.blueZone.demo.codeAriaLabel')">
                <div class="blue-code-head">
                  <span>{{ currentDemo.file }}</span>
                  <span class="blue-code-dots" aria-hidden="true"><i></i><i></i><i></i></span>
                </div>
                <pre aria-hidden="true"><code>{{ displayedCode }}</code><span v-if="showCaret" class="blue-typing-caret"></span></pre>
                <pre class="sr-only">{{ t(currentDemo.promptKey) }}. {{ currentDemo.code }}</pre>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section id="blue-scale" class="blue-section" :aria-labelledby="scaleTitleId">
        <div class="blue-container">
          <div class="blue-section-heading">
            <span class="blue-section-mark" aria-hidden="true"></span>
            <h2 :id="scaleTitleId">{{ t('home.blueZone.scale.title') }}</h2>
          </div>
          <div class="blue-metrics">
            <div class="blue-metric">
              <p class="blue-metric-value">{{ t('home.blueZone.scale.requestsValue') }}</p>
              <p class="blue-metric-label">{{ t('home.blueZone.scale.requestsLabel') }}</p>
            </div>
            <span class="blue-metric-divider" aria-hidden="true"></span>
            <div class="blue-metric">
              <p class="blue-metric-value">{{ t('home.blueZone.scale.tokensValue') }}</p>
              <p class="blue-metric-label">{{ t('home.blueZone.scale.tokensLabel') }}</p>
            </div>
          </div>
        </div>
      </section>

      <section id="blue-advantages" class="blue-section" :aria-labelledby="advantagesTitleId">
        <div class="blue-container">
          <p class="blue-section-kicker">{{ t('home.blueZone.advantages.kicker') }}</p>
          <div class="blue-section-heading">
            <span class="blue-section-mark" aria-hidden="true"></span>
            <h2 :id="advantagesTitleId">{{ t('home.blueZone.advantages.title') }}</h2>
          </div>

          <div class="blue-advantages-grid">
            <article class="blue-advantage">
              <div class="blue-feature-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor">
                  <path d="m24 6 17 8.5L24 23 7 14.5 24 6Z" />
                  <path d="m7 24 17 8.5L41 24M7 33.5 24 42l17-8.5" />
                </svg>
              </div>
              <h3>{{ t('home.blueZone.advantages.unified.title') }}</h3>
              <p>{{ t('home.blueZone.advantages.unified.description') }}</p>
            </article>
            <article class="blue-advantage">
              <div class="blue-feature-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor">
                  <path d="M15 33h-3a8 8 0 0 1 0-16 13 13 0 0 1 25-3 9 9 0 0 1-1 18h-3" />
                  <path d="M19 23v17m0 0-5-5m5 5 5-5M29 40V23m0 0-5 5m5-5 5 5" />
                </svg>
              </div>
              <h3>{{ t('home.blueZone.advantages.scheduling.title') }}</h3>
              <p>{{ t('home.blueZone.advantages.scheduling.description') }}</p>
            </article>
            <article class="blue-advantage">
              <div class="blue-feature-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor">
                  <path d="M6 41h36" />
                  <path d="M11 36V24h7v12M22 36V15h7v21M33 36V7h7v29" />
                </svg>
              </div>
              <h3>{{ t('home.blueZone.advantages.usage.title') }}</h3>
              <p>{{ t('home.blueZone.advantages.usage.description') }}</p>
            </article>
          </div>
        </div>
      </section>

      <section id="blue-models" class="blue-section blue-section-soft" :aria-labelledby="modelsTitleId">
        <div class="blue-container">
          <div class="blue-section-heading">
            <span class="blue-section-mark" aria-hidden="true"></span>
            <h2 :id="modelsTitleId">{{ t('home.blueZone.models.title') }}</h2>
          </div>

          <div class="blue-capabilities">
            <article class="blue-capability">
              <svg viewBox="0 0 40 40" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                <path d="m14 9-9 11 9 11M26 9l9 11-9 11M23 5l-6 30" />
              </svg>
              <div><strong>{{ t('home.blueZone.models.code.title') }}</strong><span>{{ t('home.blueZone.models.code.description') }}</span></div>
            </article>
            <article class="blue-capability">
              <svg viewBox="0 0 40 40" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                <rect x="5" y="7" width="24" height="25" rx="4" />
                <circle cx="13" cy="15" r="2" />
                <path d="m8 27 7-7 5 5 4-3 5 5M27 18h5a3 3 0 0 1 3 3v11l-4-4h-4" />
              </svg>
              <div><strong>{{ t('home.blueZone.models.multimodal.title') }}</strong><span>{{ t('home.blueZone.models.multimodal.description') }}</span></div>
            </article>
            <article class="blue-capability">
              <svg viewBox="0 0 40 40" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                <rect x="5" y="7" width="30" height="26" rx="4" />
                <circle cx="27" cy="14" r="2" />
                <path d="m9 28 8-9 6 6 4-4 4 7" />
              </svg>
              <div><strong>{{ t('home.blueZone.models.image.title') }}</strong><span>{{ t('home.blueZone.models.image.description') }}</span></div>
            </article>
          </div>

          <div class="blue-model-list" :aria-label="t('home.blueZone.models.listAriaLabel')">
            <div class="blue-model"><strong>DeepSeek</strong><span>DeepSeek V4</span></div>
            <div class="blue-model"><strong>GLM</strong><span>GLM-5.3</span></div>
            <div class="blue-model"><strong>GPT</strong><span>GPT-6.1 / GPT-6</span></div>
            <div class="blue-model"><strong>Gemini</strong><span>Gemini 3.8 Flash</span></div>
            <div class="blue-model"><strong>{{ t('home.blueZone.models.more') }}</strong><span>{{ t('home.blueZone.models.expanding') }}</span></div>
          </div>
        </div>
      </section>

      <section id="blue-agents" class="blue-section" :aria-labelledby="agentsTitleId">
        <div class="blue-container">
          <div class="blue-section-heading">
            <span class="blue-section-mark" aria-hidden="true"></span>
            <h2 :id="agentsTitleId">{{ t('home.blueZone.agents.title') }}</h2>
          </div>
          <div class="blue-agents">
            <div class="blue-agent">
              <div class="blue-agent-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="24" cy="24" r="17" />
                  <path d="M24 8v32M8 24h32M12 14l24 20M36 14 12 34" />
                </svg>
              </div>
              <strong>Codex</strong>
            </div>
            <div class="blue-agent">
              <div class="blue-agent-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M24 5v38M5 24h38M10 10l28 28M38 10 10 38" />
                </svg>
              </div>
              <strong>Claude Code</strong>
            </div>
            <div class="blue-agent">
              <div class="blue-agent-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="6" y="8" width="36" height="30" rx="5" />
                  <path d="m14 18 6 5-6 5M24 29h10" />
                </svg>
              </div>
              <strong>OpenCode</strong>
            </div>
            <div class="blue-agent">
              <div class="blue-agent-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="m17 10-11 14 11 14M31 10l11 14-11 14M16 24h16" />
                </svg>
              </div>
              <strong>Zcode</strong>
            </div>
            <div class="blue-agent">
              <div class="blue-agent-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="5" y="18" width="24" height="20" rx="7" />
                  <rect x="19" y="8" width="24" height="20" rx="7" />
                </svg>
              </div>
              <strong>WorkBuddy</strong>
            </div>
            <div class="blue-agent">
              <div class="blue-agent-icon" aria-hidden="true">
                <svg viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="4" y="10" width="40" height="28" rx="5" />
                  <path d="m12 19 5 5-5 5M22 29h10" />
                </svg>
              </div>
              <strong>DSH</strong>
              <small>DeepSeekHarness</small>
            </div>
          </div>
        </div>
      </section>

      <section class="blue-cta-section">
        <div class="blue-container">
          <div class="blue-cta-panel">
            <h2>{{ t('home.blueZone.cta.title') }}</h2>
            <p>{{ t('home.blueZone.cta.description') }}</p>
            <div class="blue-cta-actions">
              <RouterLink class="blue-button" :to="primaryDestination">{{ primaryLabel }}</RouterLink>
              <RouterLink
                v-if="!isAuthenticated && registrationEnabled"
                class="blue-button blue-button-secondary"
                to="/register"
              >
                {{ t('home.blueZone.register') }}
              </RouterLink>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="blue-footer">
      <div class="blue-container blue-footer-inner">
        <div class="blue-footer-brand">
          <span>{{ t('home.blueZone.brand') }}</span>
          <span>{{ t('home.blueZone.footer.internalOnly') }}</span>
        </div>
        <span>© {{ currentYear }} {{ t('home.blueZone.brand') }}</span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { BLUE_ZONE_CODE_DEMOS } from './blueZoneHomeContent'

const props = defineProps<{
  isAuthenticated: boolean
  dashboardPath: string
  isDark: boolean
  registrationEnabled: boolean
  showModelPlazaEntry: boolean
}>()

const emit = defineEmits<{
  toggleTheme: []
}>()

const { t } = useI18n()
const menuOpen = ref(false)
const assistantCardRef = ref<HTMLElement | null>(null)
const currentExampleIndex = ref(0)
const displayedCode = ref(BLUE_ZONE_CODE_DEMOS[0].code)
const showCaret = ref(true)
const isSwitching = ref(false)
const currentYear = new Date().getFullYear()

const heroTitleId = 'blue-hero-title'
const assistantTitleId = 'blue-assistant-title'
const scaleTitleId = 'blue-scale-title'
const advantagesTitleId = 'blue-advantages-title'
const modelsTitleId = 'blue-models-title'
const agentsTitleId = 'blue-agents-title'

const primaryDestination = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))
const primaryLabel = computed(() =>
  props.isAuthenticated ? t('home.goToDashboard') : t('home.blueZone.login'),
)
const currentDemo = computed(() => BLUE_ZONE_CODE_DEMOS[currentExampleIndex.value])

const timers = new Set<number>()
let observer: IntersectionObserver | null = null
let started = false
let visible = true
let characterIndex = 0

function schedule(callback: () => void, delay: number) {
  const timer = window.setTimeout(() => {
    timers.delete(timer)
    callback()
  }, delay)
  timers.add(timer)
}

function typeNextCharacter() {
  if (!visible) {
    schedule(typeNextCharacter, 240)
    return
  }

  const demo = currentDemo.value
  if (characterIndex >= demo.code.length) {
    schedule(showNextExample, 1900)
    return
  }

  const character = demo.code[characterIndex]
  displayedCode.value += character
  characterIndex += 1
  schedule(typeNextCharacter, character === '\n' ? 52 : character === ' ' ? 7 : 13)
}

function showNextExample() {
  if (!visible) {
    schedule(showNextExample, 240)
    return
  }

  currentExampleIndex.value = (currentExampleIndex.value + 1) % BLUE_ZONE_CODE_DEMOS.length
  isSwitching.value = true
  schedule(() => {
    displayedCode.value = ''
    characterIndex = 0
    isSwitching.value = false
    typeNextCharacter()
  }, 160)
}

function startExamples() {
  if (started) return
  started = true
  currentExampleIndex.value = 0
  displayedCode.value = ''
  characterIndex = 0
  schedule(typeNextCharacter, 360)
}

function closeMenu() {
  menuOpen.value = false
}

onMounted(() => {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (reducedMotion) {
    displayedCode.value = currentDemo.value.code
    showCaret.value = false
    return
  }

  if ('IntersectionObserver' in window && assistantCardRef.value) {
    observer = new IntersectionObserver(
      (entries) => {
        visible = entries.some((entry) => entry.isIntersecting)
        if (visible) startExamples()
      },
      { threshold: 0.35 },
    )
    observer.observe(assistantCardRef.value)
  } else {
    startExamples()
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  for (const timer of timers) window.clearTimeout(timer)
  timers.clear()
})
</script>

<style scoped src="./BlueZoneHome.css"></style>
