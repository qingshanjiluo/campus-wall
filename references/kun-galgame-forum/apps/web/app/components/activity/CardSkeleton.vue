<script setup lang="ts">
// Generic feed-card placeholder, shown while an infinite-scroll page is loading
// (preserves layout + reads faster than a bare spinner). Mirrors the card shell:
// an avatar circle + a name/time row + a title bar + a couple of content lines.
</script>

<template>
  <div class="flex gap-3">
    <KunSkeleton variant="circle" width="2.5rem" height="2.5rem" />
    <div class="min-w-0 flex-1 space-y-2.5 pt-1">
      <div class="flex items-center gap-2">
        <KunSkeleton variant="text" width="5rem" />
        <KunSkeleton variant="text" width="3.5rem" />
      </div>
      <KunSkeleton variant="text" width="65%" height="1.1rem" />
      <KunSkeleton variant="text" />
      <KunSkeleton variant="text" width="80%" />
    </div>
  </div>
</template>
