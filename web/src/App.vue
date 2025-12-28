<template>
  <div id="app">
    <header class="header">
      <h1>🌐 DN42 Globalping</h1>
      <span class="connection-status" :class="{ connected: isConnected }">
        {{ isConnected ? '● Connected' : '○ Disconnected' }}
      </span>
    </header>

    <div class="main-content">
      <!-- Map Section -->
      <div class="map-section">
        <h2>Probe Locations</h2>
        <div id="map" ref="mapContainer"></div>
      </div>

      <!-- Control Panel -->
      <div class="control-panel">
        <h2>Network Probe</h2>

        <!-- Probe Selection -->
        <div class="form-group">
          <label>Select Probes:</label>
          <div class="probe-list">
            <div v-if="probes.length === 0" class="no-probes">
              No probes available
            </div>
            <label
              v-for="probe in probes"
              :key="probe.id"
              class="probe-item"
            >
              <input
                type="checkbox"
                :value="probe.id"
                v-model="selectedProbes"
              />
              <span class="probe-name">{{ probe.name }}</span>
              <span class="probe-location">{{ probe.location }}</span>
            </label>
          </div>
        </div>

        <!-- Tool Selection -->
        <div class="form-group">
          <label>Tool:</label>
          <select v-model="selectedTool">
            <option value="ping">Ping</option>
            <option value="traceroute">Traceroute</option>
            <option value="mtr">MTR</option>
          </select>
        </div>

        <!-- Target Input -->
        <div class="form-group">
          <label>Target:</label>
          <input
            type="text"
            v-model="target"
            placeholder="e.g., 8.8.8.8 or google.com"
          />
        </div>

        <!-- Execute Button -->
        <button
          class="execute-btn"
          @click="executeTask"
          :disabled="!canExecute"
        >
          🚀 Execute
        </button>
      </div>

      <!-- Results Section -->
      <div class="results-section">
        <h2>Results</h2>
        <div class="results-container">
          <div
            v-for="(result, probeId) in results"
            :key="probeId"
            class="result-box"
          >
            <div class="result-header">
              <span class="probe-name">{{ result.probeName || probeId }}</span>
              <span class="status" :class="{ completed: result.completed }">
                {{ result.completed ? '✓ Done' : '⟳ Running...' }}
              </span>
            </div>
            <pre class="result-output">{{ result.output }}</pre>
          </div>
          <div v-if="Object.keys(results).length === 0" class="no-results">
            Execute a task to see results here
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'
import L from 'leaflet'

export default {
  name: 'App',
  setup() {
    const ws = ref(null)
    const isConnected = ref(false)
    const probes = ref([])
    const selectedProbes = ref([])
    const selectedTool = ref('ping')
    const target = ref('')
    const results = reactive({})
    const mapContainer = ref(null)
    let map = null
    let markers = []

    const canExecute = computed(() => {
      return selectedProbes.value.length > 0 && target.value.trim() !== ''
    })

    const connectWebSocket = () => {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/ws/client`
      
      ws.value = new WebSocket(wsUrl)

      ws.value.onopen = () => {
        console.log('WebSocket connected')
        isConnected.value = true
      }

      ws.value.onclose = () => {
        console.log('WebSocket disconnected')
        isConnected.value = false
        // Reconnect after 3 seconds
        setTimeout(connectWebSocket, 3000)
      }

      ws.value.onerror = (error) => {
        console.error('WebSocket error:', error)
      }

      ws.value.onmessage = (event) => {
        const msg = JSON.parse(event.data)
        handleMessage(msg)
      }
    }

    const handleMessage = (msg) => {
      switch (msg.type) {
        case 'probe_list':
          probes.value = msg.payload.probes || []
          updateMapMarkers()
          break
        case 'task_stream':
          handleTaskStream(msg.payload)
          break
        case 'error':
          console.error('Server error:', msg.payload.message)
          break
      }
    }

    const handleTaskStream = (payload) => {
      const { probe_id, probe_name, line, is_end, error } = payload

      if (!results[probe_id]) {
        results[probe_id] = {
          probeName: probe_name,
          output: '',
          completed: false
        }
      }

      if (line) {
        results[probe_id].output += line + '\n'
      }

      if (error) {
        results[probe_id].output += `Error: ${error}\n`
      }

      if (is_end) {
        results[probe_id].completed = true
      }
    }

    const executeTask = () => {
      if (!canExecute.value) return

      // Clear previous results
      Object.keys(results).forEach(key => delete results[key])

      const msg = {
        type: 'task_create',
        payload: {
          probe_ids: selectedProbes.value,
          type: selectedTool.value,
          target: target.value.trim()
        }
      }

      ws.value.send(JSON.stringify(msg))
    }

    const initMap = async () => {
      await nextTick()
      if (!mapContainer.value) return

      map = L.map(mapContainer.value).setView([30, 0], 2)

      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; OpenStreetMap contributors'
      }).addTo(map)
    }

    const updateMapMarkers = () => {
      if (!map) return

      // Remove existing markers
      markers.forEach(marker => map.removeLayer(marker))
      markers = []

      // Add new markers
      probes.value.forEach(probe => {
        if (probe.latitude && probe.longitude) {
          const marker = L.marker([probe.latitude, probe.longitude])
            .addTo(map)
            .bindPopup(`<b>${probe.name}</b><br>${probe.location}`)
          markers.push(marker)
        }
      })

      // Fit bounds if there are markers
      if (markers.length > 0) {
        const group = L.featureGroup(markers)
        map.fitBounds(group.getBounds().pad(0.1))
      }
    }

    onMounted(() => {
      initMap()
      connectWebSocket()
    })

    onUnmounted(() => {
      if (ws.value) {
        ws.value.close()
      }
      if (map) {
        map.remove()
      }
    })

    return {
      isConnected,
      probes,
      selectedProbes,
      selectedTool,
      target,
      results,
      canExecute,
      executeTask,
      mapContainer
    }
  }
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
  background-color: #1a1a2e;
  color: #eee;
  min-height: 100vh;
}

#app {
  min-height: 100vh;
}

.header {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem 2rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header h1 {
  font-size: 1.5rem;
}

.connection-status {
  font-size: 0.9rem;
  color: #ff6b6b;
}

.connection-status.connected {
  color: #51cf66;
}

.main-content {
  display: grid;
  grid-template-columns: 1fr 300px;
  grid-template-rows: 300px 1fr;
  gap: 1rem;
  padding: 1rem;
  max-width: 1400px;
  margin: 0 auto;
}

.map-section {
  grid-column: 1;
  grid-row: 1;
  background: #16213e;
  border-radius: 8px;
  padding: 1rem;
}

.map-section h2 {
  margin-bottom: 0.5rem;
  font-size: 1rem;
  color: #aaa;
}

#map {
  height: 230px;
  border-radius: 4px;
}

.control-panel {
  grid-column: 2;
  grid-row: 1 / 3;
  background: #16213e;
  border-radius: 8px;
  padding: 1rem;
}

.control-panel h2 {
  margin-bottom: 1rem;
  font-size: 1rem;
  color: #aaa;
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-size: 0.9rem;
  color: #888;
}

.probe-list {
  max-height: 120px;
  overflow-y: auto;
  background: #0f0f23;
  border-radius: 4px;
  padding: 0.5rem;
}

.no-probes {
  color: #666;
  font-size: 0.9rem;
  padding: 0.5rem;
}

.probe-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.3rem;
  cursor: pointer;
  border-radius: 4px;
}

.probe-item:hover {
  background: #1a1a2e;
}

.probe-item .probe-name {
  font-weight: 500;
}

.probe-item .probe-location {
  font-size: 0.8rem;
  color: #666;
  margin-left: auto;
}

select, input[type="text"] {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid #333;
  border-radius: 4px;
  background: #0f0f23;
  color: #eee;
  font-size: 0.9rem;
}

select:focus, input[type="text"]:focus {
  outline: none;
  border-color: #667eea;
}

.execute-btn {
  width: 100%;
  padding: 0.75rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
  border-radius: 4px;
  color: white;
  font-size: 1rem;
  cursor: pointer;
  transition: opacity 0.2s;
}

.execute-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.execute-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.results-section {
  grid-column: 1;
  grid-row: 2;
  background: #16213e;
  border-radius: 8px;
  padding: 1rem;
  min-height: 300px;
}

.results-section h2 {
  margin-bottom: 0.5rem;
  font-size: 1rem;
  color: #aaa;
}

.results-container {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-height: calc(100vh - 500px);
  overflow-y: auto;
}

.result-box {
  background: #0f0f23;
  border-radius: 4px;
  overflow: hidden;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background: #1a1a2e;
}

.result-header .probe-name {
  font-weight: 500;
}

.result-header .status {
  font-size: 0.8rem;
  color: #ffa94d;
}

.result-header .status.completed {
  color: #51cf66;
}

.result-output {
  padding: 1rem;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.85rem;
  line-height: 1.4;
  white-space: pre-wrap;
  overflow-wrap: break-word;
  max-height: 300px;
  overflow-y: auto;
  color: #51cf66;
}

.no-results {
  color: #666;
  text-align: center;
  padding: 2rem;
}

/* Scrollbar styling */
::-webkit-scrollbar {
  width: 6px;
}

::-webkit-scrollbar-track {
  background: #0f0f23;
}

::-webkit-scrollbar-thumb {
  background: #333;
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: #444;
}
</style>
