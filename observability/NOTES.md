# Notes





https://www.datadoghq.com/blog/go-memory-metrics/
https://docs.dynatrace.com/docs/ingest-from/technology-support/application-software/go/configuration-and-analysis/analyze-go-metrics


memory usage = /memory/classes/total:bytes − /memory/classes/heap/released:bytes

/memory/classes/heap/objects:bytes	        HeapAlloc	            Heap Live & Dead Objects  memory_classes_heap_objects_bytes
/memory/classes/heap/free:bytes	            HeapIdle - HeapReleased	Heap Reserve              memory_classes_heap_free_bytes
/memory/classes/heap/unused:bytes	        HeapInuse - HeapAlloc	Heap Reserve              memory_classes_heap_unused_bytes
/memory/classes/heap/stacks:bytes	        StackInuse	            Goroutine Stacks          memory_classes_heap_stacks_bytes
/memory/classes/os-stacks:bytes	            StackSys - StackInuse	OS Thread Stacks          memory_classes_os_stacks_bytes
/memory/classes/metadata/other:bytes	    GCSys	                GC Overhead               memory_classes_metadata_other_bytes
/memory/classes/metadata/mspan/inuse:bytes	MSpanInuse	            Allocator Overhead        memory_classes_metadata_mspan_inuse_bytes
/memory/classes/metadata/mspan/free:bytes	MSpanSys - MSpanInuse	Allocator Overhead        memory_classes_metadata_mspan_free_bytes
/memory/classes/metadata/mcache/inuse:bytes	MCacheInuse	            Allocator Overhead        memory_classes_metadata_mcache_inuse_bytes
/memory/classes/metadata/mcache/free:bytes	MCacheSys - MCacheInuse	Allocator Overhead        memory_classes_metadata_mcache_free_bytes
/memory/classes/other:bytes	                OtherSys	            Runtime Overhead          memory_classes_other_bytes
/memory/classes/profiling/buckets:bytes	    BuckHashSys	            Profiler Overhead         memory_classes_profiling_buckets_bytes


Heap Total= Heap Live & Dead Objects + Heap Reserve

Histograms
HIST - /gc/heap/allocs-by-size:bytes
HIST - /sched/pauses/stopping/other:seconds
HIST - /sched/latencies:seconds
HIST - /gc/pauses:seconds
HIST - /sched/pauses/stopping/gc:seconds
HIST - /sched/pauses/total/other:seconds
HIST - /gc/heap/frees-by-size:bytes
HIST - /sched/pauses/total/gc:seconds

some other ideas

X live gorouteins
garbage collections
X heap memory allocated
live and dead heap objects
gc target heap size
number of cgo calls
gc stw pauses per secod
gc stw pause durations
X  heap objects allocated
? heap objects freed
user forced garbage collections



USELESS?:
cpu_classes_total_cpu_seconds 42076.80772842
cpu_classes_user_cpu_seconds 59.668845257
process_cpu_seconds_total 67.99


http metrics

-------------------------
cgo_go_to_c_calls_calls 1
cpu_classes_gc_mark_assist_cpu_seconds 0.516056334
cpu_classes_gc_mark_dedicated_cpu_seconds 5.478280697
cpu_classes_gc_mark_idle_cpu_seconds 3.43630815
cpu_classes_gc_pause_cpu_seconds 6.346365809
cpu_classes_gc_total_cpu_seconds 15.77701099
cpu_classes_idle_cpu_seconds 42000.619308131
cpu_classes_scavenge_assist_cpu_seconds 1.076e-06
cpu_classes_scavenge_background_cpu_seconds 0.742562966
cpu_classes_scavenge_total_cpu_seconds 0.742564042
gc_cycles_automatic_gc_cycles 1668
gc_cycles_forced_gc_cycles 0
gc_cycles_total_gc_cycles 1668
gc_gogc_percent 100
gc_gomemlimit_bytes 9.223372036854776e+18
gc_heap_allocs_by_size_bytes 25
gc_heap_allocs_bytes 2.540644864e+09
gc_heap_allocs_objects 6.902251e+06
gc_heap_frees_by_size_bytes 25
gc_heap_frees_bytes 2.536772904e+09
gc_heap_frees_objects 6.89687e+06
gc_heap_goal_bytes 6.928906e+06
gc_heap_live_bytes 3.334568e+06
gc_heap_objects_objects 5381
gc_heap_tiny_allocs_objects 1.191409e+06
gc_limiter_last_enabled_gc_cycle 0
gc_pauses_seconds 0.000114688
gc_scan_globals_bytes 232890
gc_scan_heap_bytes 1.066144e+06
gc_scan_stack_bytes 26880
gc_scan_total_bytes 1.26701e+06
gc_stack_starting_size_bytes 2048
go_gc_duration_seconds_count 1668
go_gc_duration_seconds{quantile="0.25"} 0.000126933
go_gc_duration_seconds{quantile="0"} 4.2592e-05
go_gc_duration_seconds{quantile="0.5"} 0.000261388
go_gc_duration_seconds{quantile="0.75"} 0.000353559
go_gc_duration_seconds{quantile="1"} 0.001265035
go_gc_duration_seconds_sum 0.622115077
go_gc_gogc_percent 100
go_gc_gomemlimit_bytes 9.223372036854776e+18

go_info{version="go1.25.4"} 1
go_memstats_alloc_bytes 3.821624e+06
go_memstats_alloc_bytes_total 2.540594528e+09
go_memstats_buck_hash_sys_bytes 1.443842e+06
go_memstats_frees_total 8.088279e+06
go_memstats_gc_sys_bytes 2.972944e+06
go_memstats_heap_alloc_bytes 3.821624e+06
go_memstats_heap_idle_bytes 5.398528e+06
go_memstats_heap_inuse_bytes 6.037504e+06
go_memstats_heap_objects 5317
go_memstats_heap_released_bytes 4.58752e+06
go_memstats_heap_sys_bytes 1.1436032e+07
go_memstats_last_gc_time_seconds 1.7635134882378383e+09
go_memstats_mallocs_total 8.093596e+06
go_memstats_mcache_inuse_bytes 14496
go_memstats_mcache_sys_bytes 15704
go_memstats_mspan_inuse_bytes 228000
go_memstats_mspan_sys_bytes 261120
go_memstats_next_gc_bytes 6.928906e+06
go_memstats_other_sys_bytes 1.686174e+06
go_memstats_stack_inuse_bytes 1.14688e+06
go_memstats_stack_sys_bytes 1.14688e+06
go_memstats_sys_bytes 1.8962696e+07
go_sched_gomaxprocs_threads 12
process_max_fds 1.048576e+06
process_network_receive_bytes_total 2.87788637887e+11
process_network_transmit_bytes_total 3.19414249982e+11
process_open_fds 22
process_start_time_seconds 1.763504552e+09
process_virtual_memory_bytes 2.403618816e+09
process_resident_memory_bytes 1.972224e+07
process_virtual_memory_max_bytes 1.8446744073709552e+19
promhttp_metric_handler_requests_in_flight 1
promhttp_metric_handler_requests_total{code="200"} 3442
promhttp_metric_handler_requests_total{code="500"} 0
promhttp_metric_handler_requests_total{code="503"} 0
sched_gomaxprocs_threads 12
sched_goroutines_goroutines 6
sched_latencies_seconds 0
sched_pauses_stopping_gc_seconds 5.7344e-05
sched_pauses_stopping_other_seconds -Inf
sched_pauses_total_gc_seconds 0.000114688
sched_pauses_total_other_seconds -Inf
sync_mutex_wait_total_seconds 0.891674272
