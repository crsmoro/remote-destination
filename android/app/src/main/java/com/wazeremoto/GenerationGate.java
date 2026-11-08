package com.wazeremoto;

import java.util.concurrent.CancellationException;

/** Serializes local state and launch decisions, never network I/O. */
final class GenerationGate {
    private final Object lock;
    private long generation;
    GenerationGate(Object lock) { this.lock = lock; }
    long current() { synchronized (lock) { return generation; } }
    long invalidate() { synchronized (lock) { return ++generation; } }
    void invalidateIfCurrent(long value) { synchronized (lock) { if (generation == value) ++generation; } }
    boolean isCurrent(long value) { synchronized (lock) { return generation == value; } }
    void runIfCurrent(long value, Runnable action) { synchronized (lock) { if (generation == value) action.run(); } }
    void check(long value) throws InterruptedException {
        if (Thread.currentThread().isInterrupted() || generation != value) throw new InterruptedException("cancelled");
    }
    void process(long value, CommandProcessor.Store store, CommandProcessor.Api api, CommandProcessor.Launcher launcher) throws Exception {
        CommandProcessor.Store guarded = new CommandProcessor.Store() {
            private void checkState() { if (Thread.currentThread().isInterrupted() || generation != value) throw new CancellationException(); }
            public String attempted() { synchronized (lock) { checkState(); return store.attempted(); } }
            public String result() { synchronized (lock) { checkState(); return store.result(); } }
            public boolean pending() { synchronized (lock) { checkState(); return store.pending(); } }
            public void save(String id, String result, boolean pending) throws Exception { synchronized (lock) { check(value); store.save(id, result, pending); } }
        };
        CommandProcessor.Api guardedApi = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() throws Exception { synchronized (lock) { check(value); } CommandProcessor.Command c = api.poll(); synchronized (lock) { check(value); } return c; }
            public boolean claim(String id) throws Exception { synchronized (lock) { check(value); } boolean claimed = api.claim(id); synchronized (lock) { check(value); } return claimed; }
            public void ack(String id, String result) throws Exception { synchronized (lock) { check(value); } api.ack(id, result); synchronized (lock) { check(value); } }
        };
        new CommandProcessor(guarded).process(guardedApi, uri -> { synchronized (lock) { check(value); launcher.launch(uri); } });
    }
}
