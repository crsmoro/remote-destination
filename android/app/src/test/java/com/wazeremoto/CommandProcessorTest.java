package com.wazeremoto;

import org.junit.Test;
import static org.junit.Assert.*;
import java.util.*;

public class CommandProcessorTest {
    @Test public void interruptedDuringPollDoesNotClaimOrLaunch() throws Exception {
        java.util.concurrent.CountDownLatch entered = new java.util.concurrent.CountDownLatch(1), release = new java.util.concurrent.CountDownLatch(1);
        List<String> events = Collections.synchronizedList(new ArrayList<>());
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { entered.countDown(); while (true) { try { release.await(); break; } catch (InterruptedException ignored) { Thread.currentThread().interrupt(); break; } } return new CommandProcessor.Command("a", "uri"); }
            public boolean claim(String id) { events.add("claim"); return true; }
            public void ack(String id, String result) { events.add("ack"); }
        };
        Thread worker = new Thread(() -> { try { new CommandProcessor().process(api, uri -> events.add("launch")); } catch (Exception ignored) { } });
        worker.start(); assertTrue(entered.await(2, java.util.concurrent.TimeUnit.SECONDS));
        worker.interrupt(); release.countDown(); worker.join(2000);
        assertFalse(worker.isAlive()); assertTrue(events.toString(), events.isEmpty());
    }
    @Test public void interruptedLaunchRemainsUncertainAndIsNotAcknowledged() throws Exception {
        CommandProcessor.MemoryStore store = new CommandProcessor.MemoryStore();
        List<String> events = new ArrayList<>();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("a", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { return true; }
            public void ack(String id, String result) { events.add(result); }
        };
        try { new CommandProcessor(store).process(api, uri -> { throw new InterruptedException(); }); }
        catch (InterruptedException expected) { }
        assertTrue(events.isEmpty());
        assertNull(store.result());
        assertEquals("a", store.attempted());
    }
    @Test public void uncertainAttemptBlocksLaterCommandsUntilExplicitResolution() throws Exception {
        CommandProcessor.MemoryStore store = new CommandProcessor.MemoryStore();
        store.save("old", null, false);
        List<String> events = new ArrayList<>();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { events.add("poll"); return new CommandProcessor.Command("new", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { events.add("claim"); return true; }
            public void ack(String id, String result) { events.add("ack"); }
        };
        new CommandProcessor(store).process(api, uri -> events.add("launch"));
        assertTrue(events.isEmpty());
        assertEquals("old", store.attempted());
    }
    @Test public void retriesAckWithoutRelaunchAfterNetworkFailure() throws Exception {
        List<String> events = new ArrayList<>();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            int calls;
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("a", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { return true; }
            public void ack(String id, String result) throws Exception { events.add("ack"); if (calls++ == 0) throw new Exception("network"); }
        };
        CommandProcessor processor = new CommandProcessor();
        try { processor.process(api, uri -> events.add("launch")); fail(); } catch (Exception expected) { }
        processor.process(api, uri -> events.add("launch"));
        assertEquals(Arrays.asList("launch", "ack", "ack"), events);
    }
    @Test public void failedAckMustSurviveProcessorRestart() throws Exception {
        List<String> events = new ArrayList<>();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            int calls;
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("a", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { return true; }
            public void ack(String id, String result) throws Exception { events.add(result); if (calls++ == 0) throw new Exception(); }
        };
        CommandProcessor.MemoryStore store = new CommandProcessor.MemoryStore();
        try { new CommandProcessor(store).process(api, uri -> { throw new Exception(); }); } catch (Exception expected) { }
        new CommandProcessor(store).process(api, uri -> events.add("relaunch"));
        assertEquals(Arrays.asList("failed", "failed"), events);
    }
    @Test public void uncertainInterruptedAttemptIsNotRelaunchedOrAcknowledged() throws Exception {
        List<String> events = new ArrayList<>();
        CommandProcessor.MemoryStore store = new CommandProcessor.MemoryStore();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("a", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { return true; }
            public void ack(String id, String result) { events.add("ack:" + result); }
        };
        try { new CommandProcessor(store).process(api, uri -> { throw new AssertionError("simulated process death"); }); }
        catch (AssertionError expected) { }
        new CommandProcessor(store).process(api, uri -> events.add("relaunch"));
        assertTrue(events.isEmpty());
        assertEquals("a", store.attempted());
        assertNull(store.result());
    }
    @Test public void reportsFailedWhenLaunchFails() throws Exception {
        List<String> events = new ArrayList<>();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("a", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { return true; }
            public void ack(String id, String result) { events.add(result); }
        };
        CommandProcessor.step(api, uri -> { throw new Exception("unavailable"); });
        assertEquals(Collections.singletonList("failed"), events);
    }
    @Test public void claimPrecedesLaunchAndAcknowledgement() throws Exception {
        List<String> events = new ArrayList<>();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { events.add("poll"); return new CommandProcessor.Command("a", "https://waze.com/ul?q=x&navigate=yes"); }
            public boolean claim(String id) { events.add("claim:"+id); return true; }
            public void ack(String id, String result) { events.add("ack:"+id+":"+result); }
        };
        CommandProcessor.step(api, uri -> events.add("launch:"+uri));
        assertEquals(Arrays.asList("poll", "claim:a", "launch:https://waze.com/ul?q=x&navigate=yes", "ack:a:intent_sent"), events);
    }
}
