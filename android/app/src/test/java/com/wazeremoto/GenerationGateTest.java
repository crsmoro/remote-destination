package com.wazeremoto;

import org.junit.Test;
import static org.junit.Assert.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

public class GenerationGateTest {
    @Test public void queuedStartCallbackIsDiscardedAfterStopOrReconfigure() {
        GenerationGate gate = new GenerationGate(new Object()); AtomicInteger starts = new AtomicInteger();
        long old = gate.current();
        Runnable queued = () -> gate.runIfCurrent(old, starts::incrementAndGet);
        long replacement = gate.invalidate();
        queued.run();
        assertEquals(0, starts.get());
        gate.runIfCurrent(replacement, starts::incrementAndGet);
        assertEquals(1, starts.get());
        gate.invalidate();
        gate.runIfCurrent(replacement, starts::incrementAndGet);
        assertEquals(1, starts.get());
    }
    @Test public void cancelledBlockedPollCannotClaimOrLaunchAndDoesNotHoldLock() throws Exception {
        Object lock = new Object();
        GenerationGate gate = new GenerationGate(lock);
        long generation = gate.current();
        CountDownLatch entered = new CountDownLatch(1), release = new CountDownLatch(1);
        AtomicInteger claims = new AtomicInteger(), launches = new AtomicInteger();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() throws Exception { entered.countDown(); release.await(); return new CommandProcessor.Command("a", "uri"); }
            public boolean claim(String id) { claims.incrementAndGet(); return true; }
            public void ack(String id, String result) { }
        };
        Thread worker = new Thread(() -> { try { gate.process(generation, new CommandProcessor.MemoryStore(), api, uri -> launches.incrementAndGet()); } catch (Exception ignored) { } });
        worker.start(); assertTrue(entered.await(2, TimeUnit.SECONDS));
        ExecutorService executor = Executors.newSingleThreadExecutor();
        try { executor.submit(gate::invalidate).get(2, TimeUnit.SECONDS); }
        finally { release.countDown(); executor.shutdownNow(); worker.join(2000); }
        assertFalse(worker.isAlive()); assertEquals(0, claims.get()); assertEquals(0, launches.get());
    }
    @Test public void cancelledBlockedClaimCannotSaveOrLaunchAfterReconfiguration() throws Exception {
        Object lock = new Object(); GenerationGate gate = new GenerationGate(lock); long generation = gate.current();
        CountDownLatch entered = new CountDownLatch(1), release = new CountDownLatch(1);
        CommandProcessor.MemoryStore store = new CommandProcessor.MemoryStore(); AtomicInteger launches = new AtomicInteger();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("a", "uri"); }
            public boolean claim(String id) throws Exception { entered.countDown(); release.await(); return true; }
            public void ack(String id, String result) { }
        };
        Thread worker = new Thread(() -> { try { gate.process(generation, store, api, uri -> launches.incrementAndGet()); } catch (Exception ignored) { } });
        worker.start(); assertTrue(entered.await(2, TimeUnit.SECONDS));
        ExecutorService executor = Executors.newSingleThreadExecutor();
        try { executor.submit(gate::invalidate).get(2, TimeUnit.SECONDS); }
        finally { release.countDown(); executor.shutdownNow(); worker.join(2000); }
        assertFalse(worker.isAlive()); assertNull(store.attempted()); assertEquals(0, launches.get());
    }
}
