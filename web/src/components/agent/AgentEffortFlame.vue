<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'

const props = defineProps<{ active: boolean; ratio: number; atLimit: boolean }>()
const canvas = ref<HTMLCanvasElement | null>(null)
let cleanup: (() => void) | undefined

const fragment = `
#ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
#else
precision mediump float;
#endif
uniform vec2 size;
uniform float scale;
uniform float ratio;
uniform float limit;
uniform float clock;
uniform float dark;

float noise(vec2 point) {
  vec2 cell = floor(point);
  vec2 blend = fract(point);
  blend = blend * blend * (3.0 - 2.0 * blend);
  vec4 seeds = mod(cell.x + vec4(0.0, 1.0, 0.0, 1.0) + (cell.y + vec4(0.0, 0.0, 1.0, 1.0)) * 19.0, 289.0);
  vec4 randoms = fract(sin(seeds) * 437.7);
  return mix(mix(randoms.x, randoms.y, blend.x), mix(randoms.z, randoms.w, blend.x), blend.y);
}

void main() {
  vec2 point = gl_FragCoord.xy / scale;
  float nozzle = mix(14.0 + ratio * (size.x - 28.0), size.x, limit);
  float length = max(nozzle, 1.0);
  float distance = nozzle - point.x;
  float vertical = point.y - size.y * 0.5;
  float travel = max(distance, 0.0) / length;
  float radius = size.y * 0.5;
  if (ratio <= 0.0 || distance < 0.0 || travel >= 1.0) {
    gl_FragColor = vec4(0.0);
    return;
  }

  float stream = clock * mix(1.5, 5.6, ratio);
  vec2 sampleAt = vec2(distance * 0.055 - stream, vertical * 0.073);
  float curl = noise(sampleAt * 0.72) - 0.5;
  float shear = noise(sampleAt * 1.9 + vec2(14.0, curl * 1.8)) - 0.5;
  float filament = noise(sampleAt * vec2(2.7, 5.1) + vec2(6.0, shear * 2.2));
  float crossSection = vertical / radius + (curl * 0.49 + shear * 0.24) * smoothstep(0.0, 0.42, travel);
  float spread = abs(crossSection) + shear * 0.18 + filament * 0.11;
  float fade = (1.0 - 0.78 * smoothstep(0.25, 1.0, travel)) * smoothstep(0.0, 5.0, nozzle - 14.0);
  float plume = (0.25 + 0.75 * (1.0 - smoothstep(0.45, 1.1, spread))) * fade;
  float heat = smoothstep(0.28, 0.76, filament + curl * 0.31);
  float cells = pow(0.5 + 0.5 * cos(distance * 0.34 + shear * 2.6), 3.0);
  float core = (1.0 - smoothstep(0.04, 0.40, spread)) * fade * (0.51 + heat * 0.36 + cells * 0.13);
  float brightness = clamp(core * 1.48 + heat * 0.27, 0.0, 1.0);
  vec3 edge = mix(vec3(0.41, 0.45, 0.47), vec3(0.22), dark);
  vec3 body = mix(edge, vec3(1.0), brightness);
  float alpha = clamp(plume * (0.49 + heat * 0.34) + core * 0.42, 0.0, 1.0);
  gl_FragColor = vec4(body * alpha, alpha);
}
`

function compile(gl: WebGLRenderingContext, kind: number, source: string) {
  const shader = gl.createShader(kind)!
  gl.shaderSource(shader, source)
  gl.compileShader(shader)
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(shader)!)
  return shader
}

onMounted(() => {
  const surface = canvas.value!
  const context = surface.getContext('webgl', { antialias: false, depth: false, stencil: false })
  if (!context) return
  const gl = context

  const vertex = compile(gl, gl.VERTEX_SHADER, 'attribute vec2 position; void main() { gl_Position = vec4(position, 0.0, 1.0); }')
  const shader = compile(gl, gl.FRAGMENT_SHADER, fragment)
  const program = gl.createProgram()!
  gl.attachShader(program, vertex)
  gl.attachShader(program, shader)
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(program)!)
  gl.deleteShader(vertex)
  gl.deleteShader(shader)
  gl.useProgram(program)

  const buffer = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW)
  const position = gl.getAttribLocation(program, 'position')
  gl.enableVertexAttribArray(position)
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
  const uniforms = Object.fromEntries(['size', 'scale', 'ratio', 'limit', 'clock', 'dark'].map(name => [name, gl.getUniformLocation(program, name)]))

  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
  const forcedColors = window.matchMedia('(forced-colors: active)')
  const seed = Math.random() * 8
  let startedAt = 0
  let frame = 0

  function draw(timestamp: number) {
    frame = 0
    if (!props.active || document.hidden || forcedColors.matches || gl.isContextLost()) return
    gl.uniform1f(uniforms.ratio!, props.ratio)
    gl.uniform1f(uniforms.limit!, Number(props.atLimit))
    gl.uniform1f(uniforms.clock!, seed + (reduced.matches ? 0 : Math.max(0, timestamp - startedAt) / 1000))
    gl.drawArrays(gl.TRIANGLES, 0, 3)
    if (props.ratio > 0 && !reduced.matches) schedule()
  }

  function schedule() {
    if (!frame && props.active && !document.hidden && !forcedColors.matches) frame = requestAnimationFrame(draw)
  }

  function stop() {
    cancelAnimationFrame(frame)
    frame = 0
  }

  const resize = new ResizeObserver(([entry]) => {
    const { width, height } = entry!.contentRect
    if (!width || !height) return
    const scale = Math.min(window.devicePixelRatio, 2)
    surface.width = Math.round(width * scale)
    surface.height = Math.round(height * scale)
    gl.viewport(0, 0, surface.width, surface.height)
    gl.uniform2f(uniforms.size!, width, height)
    gl.uniform1f(uniforms.scale!, scale)
    schedule()
  })
  resize.observe(surface)

  const unwatchActive = watch(() => props.active, active => {
    if (active) {
      startedAt = performance.now()
      gl.uniform1f(uniforms.dark!, Number(document.body.getAttribute('arco-theme') === 'dark'))
      schedule()
    } else stop()
  }, { immediate: true })
  const unwatchPosition = watch(() => [props.ratio, props.atLimit], schedule)
  reduced.addEventListener('change', schedule)
  forcedColors.addEventListener('change', schedule)
  document.addEventListener('visibilitychange', schedule)
  cleanup = () => {
    stop()
    unwatchActive()
    unwatchPosition()
    resize.disconnect()
    reduced.removeEventListener('change', schedule)
    forcedColors.removeEventListener('change', schedule)
    document.removeEventListener('visibilitychange', schedule)
    gl.deleteBuffer(buffer)
    gl.deleteProgram(program)
  }
})

onUnmounted(() => cleanup?.())
</script>

<template><canvas ref="canvas" class="effort-flame" aria-hidden="true"></canvas></template>

<style scoped>
.effort-flame { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
@media (forced-colors: active) { .effort-flame { display: none; } }
</style>
