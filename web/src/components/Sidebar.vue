<script setup>
import CairnMark from './icons/CairnMark.vue'
import { api } from '../api.js'
import { session } from '../session.js'

defineProps({ actor: Object })

async function signOut() {
  // Revokes the session token server-side, not just the cookie, so a copied
  // cookie is dead too.
  try {
    await api.logout()
  } finally {
    session.actor = null
  }
}
</script>

<template>
  <!-- The nav moved off the top and down the side. On a column board the top
       edge is the most valuable strip on the screen -- it is where the column
       headers and their counts live -- and a horizontal nav was spending it on
       three words that never change. -->
  <nav class="side">
    <RouterLink to="/" class="brand">
      <CairnMark :size="16" class="mark" />
      <span class="wordmark">cairn</span>
    </RouterLink>

    <div class="group">
      <RouterLink to="/" class="item">
        <svg viewBox="0 0 16 16" class="ico" aria-hidden="true">
          <rect x="1.5" y="2.5" width="3.5" height="11" rx="1" />
          <rect x="6.25" y="2.5" width="3.5" height="7.5" rx="1" />
          <rect x="11" y="2.5" width="3.5" height="9.5" rx="1" />
        </svg>
        Board
      </RouterLink>

      <RouterLink to="/projects" class="item">
        <svg viewBox="0 0 16 16" class="ico" aria-hidden="true">
          <path d="M1.5 4.2a1.2 1.2 0 0 1 1.2-1.2h3l1.5 1.8h5.1a1.2 1.2 0 0 1 1.2 1.2v6a1.2 1.2 0 0 1-1.2 1.2H2.7a1.2 1.2 0 0 1-1.2-1.2z" />
        </svg>
        Projects
      </RouterLink>

      <RouterLink to="/agents" class="item">
        <svg viewBox="0 0 16 16" class="ico" aria-hidden="true">
          <rect x="2.5" y="4.5" width="11" height="8.5" rx="2.2" />
          <path d="M8 1.6v2.9" />
          <circle cx="6" cy="8.4" r="1.05" class="fill" />
          <circle cx="10" cy="8.4" r="1.05" class="fill" />
        </svg>
        Agents
      </RouterLink>
    </div>

    <!-- The design has the username as a label with nothing behind it, which
         left no way to sign out at all. Rather than turn it into a menu, the
         way out sits beside it and stays quiet. -->
    <div class="foot">
      <span class="who">{{ actor?.name }}</span>
      <button class="signout" @click="signOut">sign out</button>
    </div>
  </nav>
</template>

<style scoped>
.side {
  display: flex;
  flex-direction: column;
  gap: var(--s-8);
  height: 100dvh;
  padding: var(--s-6) var(--s-4);
  border-right: 1px solid var(--rule);
  background: var(--surface);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: var(--s-3);
  padding: 0 var(--s-3);
}
.brand:hover { color: inherit; }
.mark { color: var(--accent); }
.wordmark { font-size: 17px; letter-spacing: -0.01em; }

.group { display: flex; flex-direction: column; gap: var(--s-1); }

.item {
  display: flex;
  align-items: center;
  gap: var(--s-3);
  padding: var(--s-2) var(--s-3);
  border-radius: var(--r-sm);
  font-size: 13.5px;
  color: var(--text-muted);
  transition: background var(--motion), color var(--motion);
}
.item:hover { background: var(--accent-tint); color: var(--text); text-decoration: none; }
.item.router-link-exact-active { background: var(--accent-tint); color: var(--accent); }

/* One icon set, drawn as strokes so weight matches the type beside it. The
   two dots on the agent glyph are the only filled shapes. */
.ico {
  width: 15px;
  height: 15px;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.3;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.ico .fill { fill: currentColor; stroke: none; }

.foot {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: var(--s-1);
  padding: 0 var(--s-3);
}
.who { font-size: 12.5px; color: var(--text-dim); }

.signout {
  font: inherit;
  font-size: 12.5px;
  color: var(--text-faint);
  background: none;
  border: 0;
  padding: 0;
  text-align: left;
  cursor: pointer;
  transition: color var(--motion);
}
.signout:hover { color: var(--text-muted); }

/* Below tablet the rail lies down across the top: a 200px column is a third of
   a phone, and the board needs the width more than the nav does. */
@media (max-width: 860px) {
  .side {
    flex-direction: row;
    align-items: center;
    gap: var(--s-4);
    height: auto;
    border-right: 0;
    border-bottom: 1px solid var(--rule);
    padding: var(--s-3) var(--s-4);
  }
  .brand { margin-right: auto; }
  .wordmark { display: none; }
  .group { flex-direction: row; }
  .foot { margin-top: 0; flex-direction: row; align-items: center; gap: var(--s-3); }
}

/* On a phone the rail is a strip, and a strip has room for the icons or the
   words but not both plus a username. The labels go and the glyphs carry the
   nav; the username goes because it answers a question nobody asks fifteen
   times a day, while signing out has to stay reachable. */
@media (max-width: 560px) {
  .side { padding: var(--s-3); gap: var(--s-2); }
  .item { padding: var(--s-2); font-size: 0; }
  .ico { width: 17px; height: 17px; }
  .who { display: none; }
}
</style>
