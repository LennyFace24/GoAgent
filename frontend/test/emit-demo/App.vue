<script setup lang="ts">
import ParentComponent from './ParentComponent.vue'
</script>

<template>
  <div id="app">
    <h1>Vue 3 Emit 示例</h1>
    <p>演示子组件如何通过 emit 向父组件传递数据</p>
    <ParentComponent />
  </div>
</template>

<style>
#app {
  font-family: Arial, sans-serif;
  padding: 20px;
  max-width: 600px;
  margin: 0 auto;
}

h1 {
  color: #2c3e50;
  text-align: center;
}

p {
  text-align: center;
  color: #666;
  margin-bottom: 20px;
}
</style>
