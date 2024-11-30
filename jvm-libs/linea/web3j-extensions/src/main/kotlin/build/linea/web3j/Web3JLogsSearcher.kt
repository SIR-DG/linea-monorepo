package build.linea.web3j

import build.linea.domain.RetryConfig
import build.linea.web3j.domain.toDomain
import build.linea.web3j.domain.toWeb3j
import io.vertx.core.Vertx
import io.vertx.core.impl.ConcurrentHashSet
import linea.EthLogsSearcher
import linea.SearchDirection
import net.consensys.linea.BlockParameter
import net.consensys.linea.BlockParameter.Companion.toBlockParameter
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.toSafeFuture
import net.consensys.toULong
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthLog
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

sealed interface SearchResultT<T> {
  data class ItemFound<T>(val log: T) : SearchResultT<T>
  data class KeepSearching<T>(val direction: SearchDirection) : SearchResultT<T>
}

private sealed interface SearchResult {
  data class ItemFound(val log: build.linea.domain.EthLog) : SearchResult
  data class KeepSearching(val direction: SearchDirection) : SearchResult
}

class Web3JLogsSearcher(
  val vertx: Vertx,
  val web3jClient: Web3j,
  val config: Config = Config(),
  val log: Logger = LogManager.getLogger(Web3JLogsSearcher::class.java)
) : EthLogsSearcher {
  data class Config(
    val backoffDelay: Duration = 100.milliseconds,
    val requestRetryConfig: RetryConfig = RetryConfig()
  )

  override fun findLog(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    chunkSize: Int,
    address: String,
    topics: List<String>,
    shallContinueToSearchPredicate: (build.linea.domain.EthLog) -> SearchDirection?
  ): SafeFuture<build.linea.domain.EthLog?> {
    require(chunkSize > 0) { "chunkSize=$chunkSize must be greater than 0" }

    return getAbsoluteBlockNumbers(fromBlock, toBlock)
      .thenCompose { (start, end) ->
        findLogLoop(
          start,
          end,
          chunkSize,
          address,
          topics,
          shallContinueToSearchPredicate
        )
      }
  }

  private fun findLogLoop(
    fromBlock: ULong,
    toBlock: ULong,
    chunkSize: Int,
    address: String,
    topics: List<String>,
    shallContinueToSearchPredicate: (build.linea.domain.EthLog) -> SearchDirection?
  ): SafeFuture<build.linea.domain.EthLog?> {
    val searchChunks = (fromBlock..toBlock)
      .chunked(chunkSize)
      .map { it.first() to it.last() }
    log.debug("searching in chunks={}", searchChunks)
    val threads = ConcurrentHashSet<String>()
    val left = AtomicInteger(0)
    val right = AtomicInteger(searchChunks.size - 1)

    return AsyncRetryer.retry(
      vertx,
      backoffDelay = config.backoffDelay,
      stopRetriesPredicate = { it is SearchResult.ItemFound || left.get() > right.get() }
    ) {
      threads.add(Thread.currentThread().name)
      val mid = left.get() + (right.get() - left.get()) / 2
      val (chunkStart, chunkEnd) = searchChunks[mid]
      log.debug("searching in chunk {}..{} (left={}, right={})", chunkStart, chunkEnd, left, right)
      findLogInInterval(chunkStart, chunkEnd, address, topics, shallContinueToSearchPredicate)
        .thenPeek { result ->
          threads.add(Thread.currentThread().name)
          if (result is SearchResult.KeepSearching) {
            if (result.direction == SearchDirection.FORWARD) {
              left.set(mid + 1)
            } else {
              right.set(mid + 1)
            }
          }
        }
    }.thenApply { either ->
      when (either) {
        is SearchResult.ItemFound -> either.log
        else -> null
      }
    }
  }

  private fun findLogInInterval(
    fromBlock: ULong,
    toBlock: ULong,
    address: String,
    topics: List<String>,
    shallContinueToSearchPredicate: (build.linea.domain.EthLog) -> SearchDirection?
  ): SafeFuture<SearchResult> {
    return getLogs(
      fromBlock = fromBlock.toBlockParameter(),
      toBlock = toBlock.toBlockParameter(),
      address = address,
      topics = topics
    )
      .thenApply { logs ->
        val item = logs.find { shallContinueToSearchPredicate(it) == null }
        if (item != null) {
          SearchResult.ItemFound(item)
        } else {
          val nextSearchDirection = shallContinueToSearchPredicate(logs.first())!!
          SearchResult.KeepSearching(nextSearchDirection)
        }
      }
  }

  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<build.linea.domain.EthLog>> {
    return if (config.requestRetryConfig.isRetryEnabled) {
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.requestRetryConfig.backoffDelay,
        timeout = config.requestRetryConfig.timeout,
        maxRetries = config.requestRetryConfig.maxRetries?.toInt()
      ) {
        getLogsInternal(fromBlock, toBlock, address, topics)
      }
    } else {
      getLogsInternal(fromBlock, toBlock, address, topics)
    }
  }

  private fun getLogsInternal(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<build.linea.domain.EthLog>> {
    val ethFilter = EthFilter(
      /*fromBlock*/ fromBlock.toWeb3j(),
      /*toBlock*/ toBlock.toWeb3j(),
      /*address*/ address
    ).apply {
      topics.forEach { addSingleTopic(it) }
    }

    return web3jClient
      .ethGetLogs(ethFilter)
      .sendAsync()
      .toSafeFuture()
      .thenCompose {
        if (it.hasError()) {
          SafeFuture.failedFuture(
            RuntimeException(
              "json-rpc error: code=${it.error.code} message=${it.error.message} " +
                "data=${it.error.data}"
            )
          )
        } else {
          val logs = if (it.logs != null) {
            @Suppress("UNCHECKED_CAST")
            (it.logs as List<EthLog.LogResult<Log>>)
              .map { logResult ->
                logResult.get().toDomain()
              }
          } else {
            emptyList()
          }

          SafeFuture.completedFuture(logs)
        }
      }
  }

  private fun getAbsoluteBlockNumbers(
    fromBlock: BlockParameter,
    toBlock: BlockParameter
  ): SafeFuture<Pair<ULong, ULong>> {
    return if (fromBlock is BlockParameter.BlockNumber && toBlock is BlockParameter.BlockNumber) {
      return SafeFuture.completedFuture(Pair(fromBlock.getNumber(), toBlock.getNumber()))
    } else {
      SafeFuture.collectAll(
        web3jClient.ethGetBlockByNumber(fromBlock.toWeb3j(), false).sendAsync().toSafeFuture(),
        web3jClient.ethGetBlockByNumber(toBlock.toWeb3j(), false).sendAsync().toSafeFuture()
      ).thenApply { (fromBlockResponse, toBlockResponse) ->
        Pair(
          fromBlockResponse.block.number.toULong(),
          toBlockResponse.block.number.toULong()
        )
      }
    }
  }
}
