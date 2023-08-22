package net.consensys.linea.traces.app

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.core.VertxOptions
import io.vertx.core.json.jackson.VertxModule
import io.vertx.micrometer.MicrometerMetricsOptions
import io.vertx.micrometer.VertxPrometheusOptions
import io.vertx.micrometer.backends.BackendRegistries
import net.consensys.linea.TracesConflationServiceV1Impl
import net.consensys.linea.TracesCountingServiceV1Impl
import net.consensys.linea.traces.RawJsonTracesConflator
import net.consensys.linea.traces.RawJsonTracesCounter
import net.consensys.linea.traces.app.api.Api
import net.consensys.linea.traces.app.api.ApiConfig
import net.consensys.linea.traces.repository.FilesystemConflatedTracesRepository
import net.consensys.linea.traces.repository.FilesystemTracesRepositoryV1
import net.consensys.linea.traces.repository.ReadTracesCacheConfig
import net.consensys.linea.vertx.loadVertxConfig
import org.apache.logging.log4j.LogManager
import java.nio.file.Files
import java.nio.file.Path

data class AppConfig(
  val inputTracesDirectory: String,
  val outputTracesDirectory: String,
  val tracesVersion: String,
  val api: ApiConfig,
  val readTracesCache: ReadTracesCacheConfig,
  val tracesFileExtension: String,
  // This is meant fo be false for local Debug only. Not in prod
  // Override in CLI with --Dconfig.override.conflated_trace_compression=false
  val conflatedTracesCompression: Boolean = true
)

class TracesApiFacadeApp(config: AppConfig) {
  private val log = LogManager.getLogger(TracesApiFacadeApp::class.java)
  private val meterRegistry: MeterRegistry
  private val vertx: Vertx
  private var api: Api

  init {
    log.debug("System properties: {}", System.getProperties())
    val vertxConfigJson = loadVertxConfig(System.getProperty("vertx.configurationFile"))
    log.info("Vertx custom configs: {}", vertxConfigJson)
    val vertxConfig =
      VertxOptions(vertxConfigJson)
        .setMetricsOptions(
          MicrometerMetricsOptions()
            .setJvmMetricsEnabled(true)
            .setPrometheusOptions(
              VertxPrometheusOptions().setPublishQuantiles(true).setEnabled(true)
            )
            .setEnabled(true)
        )
    log.debug("Vertx full configs: {}", vertxConfig)
    log.info("App configs: {}", config)
    validateConfig(config)
    this.vertx = Vertx.vertx(vertxConfig)
    this.meterRegistry = BackendRegistries.getDefaultNow()
    val tracesRepository =
      FilesystemTracesRepositoryV1(
        vertx,
        FilesystemTracesRepositoryV1.Config(
          Path.of(config.inputTracesDirectory),
          config.tracesVersion,
          config.tracesFileExtension
        )
      )
    // CachedTracesRepository breaks the application with internal error
    // when the file does not exist in the file system
    // val cachedTracesRepository = CachedTracesRepository(tracesRepository, config.readTracesCache)
    val cachedTracesRepository = tracesRepository
    val jsonSerializerObjectMapper = jacksonObjectMapper().apply {
      this.registerModule(VertxModule())
    }
    val conflatedTracesRepository =
      FilesystemConflatedTracesRepository(
        vertx,
        Path.of(config.outputTracesDirectory),
        gzipCompressionEnabled = config.conflatedTracesCompression,
        jsonSerializerObjectMapper
      )
    val tracesCounterService =
      TracesCountingServiceV1Impl(
        cachedTracesRepository,
        RawJsonTracesCounter(config.tracesVersion)
      )
    val tracesConflationService =
      TracesConflationServiceV1Impl(
        cachedTracesRepository,
        RawJsonTracesConflator(config.tracesVersion),
        conflatedTracesRepository,
        config.tracesVersion
      )
    this.api = Api(config.api, vertx, meterRegistry, tracesCounterService, tracesConflationService)
  }

  fun start(): Future<*> {
    return api.start().onComplete { log.info("App successfully started") }
  }

  fun stop(): Future<*> {
    log.info("Shooting down app..")
    return api.stop().onComplete { log.info("App successfully closed") }
  }

  private fun validateConfig(config: AppConfig): Boolean {
    assertDirectory(Path.of(config.inputTracesDirectory).toAbsolutePath())
    assertDirectory(
      Path.of(config.outputTracesDirectory).toAbsolutePath(),
      createIfDoesNotExist = true
    )
    return true
  }

  private fun assertDirectory(directory: Path, createIfDoesNotExist: Boolean = false) {
    if (!Files.exists(directory)) {
      if (createIfDoesNotExist) {
        Files.createDirectories(directory)
      } else {
        throw Exception("Directory not found: $directory")
      }
    }
    if (!Files.isReadable(directory)) throw Exception("Cannot read directory: $directory")
  }
}
