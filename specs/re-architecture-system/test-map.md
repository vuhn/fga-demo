# Collected pytest node IDs before the re-architecture (210, 2026-11-19). Every ID must end up in a collected test or a reviewed removal.

| Baseline node ID | Destination | Status |
| --- | --- | --- |
| `tests/test_app.py::test_from_text_cleans_draft` | | pending |
| `tests/test_app.py::test_revise_applies_ops_and_regenerates` | | pending |
| `tests/test_app.py::test_revise_unknown_room_is_422` | | pending |
| `tests/test_app.py::test_unsupported_request_is_422` | | pending |
| `tests/test_app.py::test_repair_only_when_requested_and_never_uses_the_llm` | | pending |
| `tests/test_app.py::test_render_formats` | | pending |
| `tests/test_app.py::test_harness_page_served` | | pending |
| `tests/test_app.py::test_schemas_are_json_schema_ready` | | pending |
| `tests/test_app.py::test_from_text_drops_sides_the_user_never_named` | | pending |
| `tests/test_app.py::test_from_text_nests_rooms_and_merges_fixtures` | | pending |
| `tests/test_app.py::test_enhancer_requirements_feed_the_draft_and_assumptions_become_notes` | | pending |
| `tests/test_app.py::test_enhancer_is_off_by_default` | | pending |
| `tests/test_app.py::test_revise_retries_with_the_error_and_succeeds` | | pending |
| `tests/test_app.py::test_revise_gives_up_after_three_attempts` | | pending |
| `tests/test_app.py::test_unsupported_is_not_retried` | | pending |
| `tests/test_app.py::test_history_reaches_the_revise_prompt` | | pending |
| `tests/test_app.py::test_reviewer_keeps_an_improving_round` | | pending |
| `tests/test_app.py::test_reviewer_refuses_forbidden_ops_and_never_returns_a_worse_plan` | | pending |
| `tests/test_app.py::test_reviewer_caps_total_area_change` | | pending |
| `tests/test_app.py::test_checks_carry_issue_ids_categories_and_hints` | | pending |
| `tests/test_app.py::test_reviewer_retries_an_answer_that_changes_nothing` | | pending |
| `tests/test_app.py::test_removing_the_hall_is_steered_to_open_plan` | | pending |
| `tests/test_app.py::test_draft_can_ask_for_open_plan` | | pending |
| `tests/test_app.py::test_revise_stream_emits_progress_then_result` | | pending |
| `tests/test_app.py::test_revise_stream_reports_retry_and_error` | | pending |
| `tests/test_app.py::test_revise_stream_empty_message_is_plain_422` | | pending |
| `tests/test_app.py::test_thinking_is_streamed_only_while_a_sink_listens` | | pending |
| `tests/test_convert.py::test_pdf_vector_areas[family_3br]` | | pending |
| `tests/test_convert.py::test_pdf_vector_areas[studio]` | | pending |
| `tests/test_convert.py::test_raster_areas_with_known_length[family_3br]` | | pending |
| `tests/test_convert.py::test_raster_areas_with_known_length[studio]` | | pending |
| `tests/test_convert.py::test_implausible_scale_is_reported` | | pending |
| `tests/test_convert.py::test_bad_input` | | pending |
| `tests/test_convert.py::test_endpoint` | | pending |
| `tests/test_convert.py::test_raster_reads_its_own_scale_and_room_names` | | pending |
| `tests/test_convert.py::test_raster_without_a_readable_scale_asks_for_one` | | pending |
| `tests/test_convert.py::test_labelled_screenshot_keeps_windows_fixtures_and_rooms` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[accumulatortest.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[blocks1.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[blocks2.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[circlesellipsesarcs.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[closedlwpolylinebug.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[dimensions.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[ellipticalarcs.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[ellipticalarcs2.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[hatches.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[layers.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[lines.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[lwpolylines.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[openscad_export.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[points.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[splineA.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[splines.dxf]` | | pending |
| `tests/test_drawing.py::test_every_sample_file_imports_and_renders[texts.dxf]` | | pending |
| `tests/test_drawing.py::test_circles_arcs_ellipses_and_splines_become_curves` | | pending |
| `tests/test_drawing.py::test_all_entity_kinds_in_one_file` | | pending |
| `tests/test_drawing.py::test_a_floor_plan_keeps_its_rooms_and_gets_the_other_entities_as_drawing_items` | | pending |
| `tests/test_drawing.py::test_a_file_with_no_entities_is_refused` | | pending |
| `tests/test_drawing.py::test_endpoint_imports_a_plain_drawing` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[accumulatortest.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[blocks1.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[circlesellipsesarcs.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[closedlwpolylinebug.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[dimensions.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[ellipticalarcs.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[ellipticalarcs2.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[hatches.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[layers.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[lines.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[lwpolylines.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[openscad_export.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[points.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[splineA.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[splines.dxf]` | | pending |
| `tests/test_drawing.py::test_a_drawing_fills_its_sheet[texts.dxf]` | | pending |
| `tests/test_estimate.py::test_takeoff_areas_and_walls_are_hand_checkable` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_existing_rooms_are_not_built_and_replaced_rooms_get_demolition` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_assumed_height_makes_dependent_lines_assumed` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_new_build_adds_structure_and_mep_points` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_a_drawing_with_no_rooms_is_unsupported_not_zero` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_cascade_to_the_dong` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_contingency_rises_with_assumed_share_and_unpriced_lines_stay_out` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_quote_must_be_on_the_page_and_carry_the_number` | | pending |
| `tests/test_estimate.py::test_units_and_currency_convert_or_are_rejected` | | pending |
| `tests/test_estimate.py::test_aggregation_drops_outliers_and_flags_thin_evidence` | | pending |
| `tests/test_estimate.py::test_web_source_accepts_real_quotes_rejects_fakes_and_caches` | | pending |
| `tests/test_estimate.py::test_web_source_with_nothing_usable_is_unpriced_and_queries_hold_no_project_data` | | pending |
| `tests/test_estimate.py::test_research_requests_carry_only_material_region_and_quality` | | pending |
| `tests/test_estimate.py::test_the_example_conversation_asks_before_it_assumes` | | pending |
| `tests/test_estimate.py::test_a_place_answer_is_split_by_code_and_an_empty_shell_is_a_fit_out` | | pending |
| `tests/test_estimate.py::test_i_dont_know_is_recorded_as_assumed_and_shown_in_the_result` | | pending |
| `tests/test_estimate.py::test_several_slots_in_one_message_and_typical_markups` | | pending |
| `tests/test_estimate.py::test_missing_slots_come_from_code_not_the_llm` | | pending |
| `tests/test_estimate.py::test_changing_an_answer_reprices_only_what_changed` | | pending |
| `tests/test_estimate.py::test_scope_changes_the_total` | | pending |
| `tests/test_estimate.py::test_unpriced_codes_are_flagged_and_a_user_rate_fills_them` | | pending |
| `tests/test_estimate.py::test_no_search_provider_means_unpriced_with_a_reason` | | pending |
| `tests/test_estimate.py::test_explaining_a_line` | | pending |
| `tests/test_estimate.py::test_drawing_without_rooms_fails_cleanly` | | pending |
| `tests/test_estimate.py::test_a_question_turn_streams_progress_tokens_question_then_done` | | pending |
| `tests/test_estimate.py::test_an_estimate_turn_streams_rates_then_the_estimate_then_the_closing_reply` | | pending |
| `tests/test_estimate.py::test_llm_failure_mid_stream_is_an_error_event` | | pending |
| `tests/test_estimate.py::test_a_dead_llm_before_any_text_falls_back_to_the_question_text` | | pending |
| `tests/test_estimate.py::test_chips_match_the_slot_asked_and_always_offer_you_decide` | | pending |
| `tests/test_estimate.py::test_flags_the_model_sets_unprompted_do_not_assume_anything` | | pending |
| `tests/test_estimate.py::test_typical_and_give_up_phrases_still_work_when_said` | | pending |
| `tests/test_estimate.py::test_the_same_scope_twice_is_one_fact_event` | | pending |
| `tests/test_estimate.py::test_extraction_calls_ask_for_no_live_thinking` | | pending |
| `tests/test_estimate.py::test_code_reads_what_the_llm_missed_and_drops_an_unprompted_scope` | | pending |
| `tests/test_estimate.py::test_the_agents_own_chips_are_understood_without_the_llm` | | pending |
| `tests/test_estimate.py::test_exist_alone_never_sets_every_room` | | pending |
| `tests/test_estimate.py::test_a_room_can_be_marked_only_when_the_message_refers_to_it` | | pending |
| `tests/test_estimate.py::test_the_region_is_pinned_by_code_not_the_model` | | pending |
| `tests/test_estimate.py::test_a_query_naming_the_wrong_city_never_reaches_the_search_provider` | | pending |
| `tests/test_estimate.py::test_the_cited_sources_are_only_those_the_rate_rests_on` | | pending |
| `tests/test_estimate.py::test_fixtures_are_assumed_absent_and_priced_unless_they_exist` | `blueprint-engine/tests/test_estimate.py` (same ID; calls the legacy `takeoff`/`price` adapters, which convert the session with `facts_of` and call the `Takeoff`/`PriceEstimate` services, T5; moves to the engine tests in T16) | pending |
| `tests/test_estimate.py::test_the_agent_asks_which_fixtures_exist_before_it_prices` | | pending |
| `tests/test_estimate.py::test_some_fixtures_exist_only_those_the_message_refers_to` | | pending |
| `tests/test_estimate.py::test_i_dont_know_assumes_no_fixture_exists_and_says_so` | | pending |
| `tests/test_estimate.py::test_demo_search_prices_every_code_through_the_real_pipeline_and_labels_it` | | pending |
| `tests/test_estimate.py::test_an_estimate_on_demo_prices_says_it_is_demo_data` | | pending |
| `tests/test_estimate.py::test_a_named_city_and_an_explicit_floor_height_never_depend_on_the_llm` | | pending |
| `tests/test_estimate.py::test_include_or_exclude_mep_is_read_by_code` | | pending |
| `tests/test_estimate.py::test_work_type_and_structure_keywords_are_read_by_code` | | pending |
| `tests/test_estimate.py::test_removing_an_item_from_a_finished_estimate_really_removes_it` | | pending |
| `tests/test_estimate.py::test_the_model_cannot_remove_items_the_message_never_mentions` | | pending |
| `tests/test_estimate.py::test_an_item_can_be_put_back` | | pending |
| `tests/test_estimate.py::test_a_message_that_changes_nothing_never_claims_a_change` | | pending |
| `tests/test_estimate.py::test_the_reply_after_a_real_change_is_given_exactly_what_changed` | | pending |
| `tests/test_estimate.py::test_an_exception_in_the_first_message_takes_that_opening_out` | | pending |
| `tests/test_estimate.py::test_exception_and_only_phrases_name_openings_only` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[except entrance door-Fixtures are almost empty, except entrance door.-True]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[EXCEPT   the Entrance Door-fixtures are empty, except the entrance door-True]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[tr\u1eeb c\u1eeda ch\xednh-Tr\u1eeb c\u1eeda ch\xednh, thi\u1ebft b\u1ecb ch\u01b0a c\xf3 c\xe1i n\xe0o-True]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[tru cua chinh-Tr\u1eeb c\u1eeda ch\xednh, thi\u1ebft b\u1ecb ch\u01b0a c\xf3-True]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[\u5165\u53e3\u306e\u30c9\u30a2\u3092\u9664\u3044\u3066-\u5165\u53e3\u306e\u30c9\u30a2\u3092\u9664\u3044\u3066\u3001\u8a2d\u5099\u306f\u4f55\u3082\u3042\u308a\u307e\u305b\u3093-True]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[typical percentages-This is an apartment in Da Nang-False]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[the doors exist-Fixtures are empty, except the entrance door-False]` | | pending |
| `tests/test_estimate.py::test_a_quote_counts_only_when_it_is_in_the_message[a-a b c-False]` | | pending |
| `tests/test_estimate.py::test_values_without_a_verified_quote_are_dropped` | | pending |
| `tests/test_estimate.py::test_the_model_reads_an_exception_in_any_language[Fixtures are almost empty, except entrance door.-except entrance door-items0]` | | pending |
| `tests/test_estimate.py::test_the_model_reads_an_exception_in_any_language[Tr\u1eeb c\u1eeda ch\xednh, thi\u1ebft b\u1ecb ch\u01b0a c\xf3 c\xe1i n\xe0o-Tr\u1eeb c\u1eeda ch\xednh-items1]` | | pending |
| `tests/test_estimate.py::test_the_model_reads_an_exception_in_any_language[\u5165\u53e3\u306e\u30c9\u30a2\u3092\u9664\u3044\u3066\u3001\u8a2d\u5099\u306f\u4f55\u3082\u3042\u308a\u307e\u305b\u3093-\u5165\u53e3\u306e\u30c9\u30a2\u3092\u9664\u3044\u3066-items2]` | | pending |
| `tests/test_estimate.py::test_the_two_readings_run_for_every_message_and_a_failure_is_not_hidden` | | pending |
| `tests/test_import.py::test_sheet_splits_into_three_plans` | | pending |
| `tests/test_import.py::test_known_answer[1]` | | pending |
| `tests/test_import.py::test_known_answer[2]` | | pending |
| `tests/test_import.py::test_known_answer[3]` | | pending |
| `tests/test_import.py::test_labels_and_types` | | pending |
| `tests/test_import.py::test_angled_wall_is_kept` | | pending |
| `tests/test_import.py::test_imported_plans_render_and_check[0]` | | pending |
| `tests/test_import.py::test_imported_plans_render_and_check[1]` | | pending |
| `tests/test_import.py::test_imported_plans_render_and_check[2]` | | pending |
| `tests/test_import.py::test_unitless_file_is_guessed_and_confirmed_by_dimensions` | | pending |
| `tests/test_import.py::test_unit_override` | | pending |
| `tests/test_import.py::test_garbage_and_empty_files_are_refused` | | pending |
| `tests/test_import.py::test_endpoint_returns_plans_with_checks_and_svg` | | pending |
| `tests/test_import.py::test_single_line_walls_are_read_with_a_default_thickness` | | pending |
| `tests/test_import.py::test_our_own_dxf_export_imports_back_to_the_same_rooms[studio-brief0]` | | pending |
| `tests/test_import.py::test_our_own_dxf_export_imports_back_to_the_same_rooms[west_entry_study-brief1]` | | pending |
| `tests/test_import.py::test_our_own_dxf_export_imports_back_to_the_same_rooms[family_3br-brief2]` | | pending |
| `tests/test_import.py::test_our_own_dxf_export_imports_back_to_the_same_rooms[studio-brief3]` | | pending |
| `tests/test_import.py::test_our_own_dxf_export_imports_back_to_the_same_rooms[north_entry_2br-brief4]` | | pending |
| `tests/test_import.py::test_round_trip_keeps_fixtures_and_reports_unsupported_items` | | pending |
| `tests/test_import.py::test_dimensions_and_other_entities_are_kept_as_drawing_items` | | pending |
| `tests/test_import.py::test_plan_revise_renames_adds_and_resizes` | | pending |
| `tests/test_import.py::test_plan_revise_adds_and_removes_fixtures` | | pending |
| `tests/test_import.py::test_plan_revise_refuses_geometry_changes_and_unknown_names` | | pending |
| `tests/test_import.py::test_plan_revise_stream_reports_result` | | pending |
| `tests/test_import.py::test_a_loose_leaf_and_arc_door_sits_in_its_wall_and_a_free_wall_end_adds_no_wall` | | pending |
| `tests/test_patch.py::test_provenance_is_recorded` | | pending |
| `tests/test_patch.py::test_no_change_leaves_the_file_as_it_was` | | pending |
| `tests/test_patch.py::test_rename_changes_only_the_label` | | pending |
| `tests/test_patch.py::test_every_opening_can_be_removed_without_merging_rooms` | | pending |
| `tests/test_patch.py::test_window_resize_keeps_the_position` | | pending |
| `tests/test_patch.py::test_fixture_move_remove_and_add` | | pending |
| `tests/test_patch.py::test_added_fixture_without_a_template_is_drawn_on_the_added_layer` | | pending |
| `tests/test_patch.py::test_unsupported_changes_are_reported_not_dropped` | | pending |
| `tests/test_patch.py::test_a_generated_plan_cannot_be_patched` | | pending |
| `tests/test_patch.py::test_direct_edit_moves_turns_and_slides_without_an_llm` | | pending |
| `tests/test_patch.py::test_a_direct_edit_then_export_writes_the_change_into_the_file` | | pending |
| `tests/test_patch.py::test_delete_fixture_by_id_and_the_chat_help_lists_the_new_ops` | | pending |
| `tests/test_patch.py::test_a_created_plan_is_an_editable_cad_plan` | | pending |
| `tests/test_patch.py::test_a_brief_that_cannot_be_built_creates_nothing` | | pending |
| `tests/test_patch.py::test_a_plan_made_from_a_brief_can_be_drawn_into_an_editable_cad_plan` | | pending |
| `tests/test_patch.py::test_an_open_plan_reads_back_as_fewer_spaces_and_says_so` | `tests/test_patch.py` (same ID; now calls the `PlanDrawer` usecase via `wire.build_engine`, T4a) | pending |
| `tests/test_pipeline.py::test_feasible_briefs_pass_every_error_check[family_3br]` | | pending |
| `tests/test_pipeline.py::test_feasible_briefs_pass_every_error_check[north_entry_2br]` | | pending |
| `tests/test_pipeline.py::test_feasible_briefs_pass_every_error_check[studio]` | | pending |
| `tests/test_pipeline.py::test_feasible_briefs_pass_every_error_check[west_entry_study]` | | pending |
| `tests/test_pipeline.py::test_infeasible_brief_reports_area_fact` | | pending |
| `tests/test_pipeline.py::test_deterministic` | | pending |
| `tests/test_pipeline.py::test_rooms_tile_the_footprint_without_overlap` | | pending |
| `tests/test_pipeline.py::test_side_constraint_and_entrance` | | pending |
| `tests/test_pipeline.py::test_revision_moves_one_room_and_keeps_the_rest_stable` | | pending |
| `tests/test_pipeline.py::test_outputs_render` | | pending |
| `tests/test_pipeline.py::test_brief_rejects_unknown_adjacency` | | pending |
| `tests/test_pipeline.py::test_nested_rooms_sit_inside_their_parent_block_and_open_only_into_it` | | pending |
| `tests/test_pipeline.py::test_nested_room_without_hall_keeps_the_door_chain` | | pending |
| `tests/test_pipeline.py::test_brief_rejects_bad_nesting` | | pending |
| `tests/test_pipeline.py::test_fixtures_default_by_type_and_stay_inside_their_room` | | pending |
| `tests/test_pipeline.py::test_explicit_empty_fixtures_and_unfit_fixture_warns` | | pending |
| `tests/test_pipeline.py::test_edit_ops_for_nesting_and_fixtures` | | pending |
| `tests/test_pipeline.py::test_add_fixture_is_idempotent` | | pending |
| `tests/test_pipeline.py::test_plans_are_drawn_at_1_to_50_on_a_sheet_that_fits` | | pending |
| `tests/test_pipeline.py::test_defensive_rules_fire_on_a_broken_model` | | pending |
| `tests/test_pipeline.py::test_design_warning_for_a_bedroom_on_the_street` | | pending |
| `tests/test_pipeline.py::test_ifc_export_matches_the_model` | | pending |
| `tests/test_pipeline.py::test_open_plan_removes_the_walls_between_hall_living_kitchen_and_dining` | | pending |
| `tests/test_pipeline.py::test_the_hall_cannot_be_edited_and_says_how_to_open_it` | | pending |

## New tests added by T5 (not in the baseline)

| Collected ID | What it covers |
| --- | --- |
| `tests/test_estimate_services.py::test_summarize_lists_rooms_fixtures_and_openings` | `SummarizePlan` rooms, fixtures, openings |
| `tests/test_estimate_services.py::test_summarize_pending_fixtures_are_priced_unknown_ones_only` | `pending_fixtures`: priced kinds only, `fixtures_existing`, existing rooms |
| `tests/test_estimate_services.py::test_summarize_structure_in_scope_follows_the_facts` | `structure_in_scope` for new build / renovation |
| `tests/test_estimate_services.py::test_summarize_an_empty_plan_is_empty` | empty plan summary |
| `tests/test_estimate_services.py::test_takeoff_and_price_take_facts_and_match_the_legacy_adapter` | services on `EstimateFacts` equal the legacy adapters |
| `tests/test_estimate_services.py::test_takeoff_honours_excluded_codes_and_the_source_of_a_fact` | `excluded`, fact source -> confidence |
| `tests/test_estimate_services.py::test_price_uses_markup_facts_and_defaults` | markup facts and defaults |
