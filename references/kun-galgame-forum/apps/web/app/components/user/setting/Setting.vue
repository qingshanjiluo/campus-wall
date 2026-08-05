<script setup lang="ts">
defineProps<{
  user: UserInfo
}>()
</script>

<template>
  <div class="h-full w-full space-y-6">
    <UserSettingAvatar />
    <UserSettingBio :user="user" />
    <UserSettingUsername />
    <UserSettingEmail />
    <UserSettingPassword />
    <KunCard :is-hoverable="false" content-class="space-y-4">
      <MessageNotificationPreference />
    </KunCard>
  </div>
</template>
