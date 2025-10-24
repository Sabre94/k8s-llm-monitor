# Combined_Greedy_NSGA2.py
import numpy as np
import matplotlib.pyplot as plt
from matplotlib.patches import Circle
from scipy.spatial.distance import cdist
import math
import time
import random
import copy # For deep copying individuals
from mpl_toolkits.mplot3d import Axes3D # For 3D plotting if needed
try:
    import pandas as pd
    from pandas.plotting import parallel_coordinates
    PANDAS_AVAILABLE = True
except ImportError:
    PANDAS_AVAILABLE = False
    print("Warning: pandas is not installed. Plotting features will be skipped.")
    print("Please install pandas: pip install pandas")

# --- Constants and Parameters ---

# Greedy Parameters (can be overridden in main section)
DEFAULT_TASK_TYPE = 'sustain'
DEFAULT_TARGET_COVERAGE = 0.90
DEFAULT_GRID_DENSITY_GREEDY = 40 # For Greedy's area calc

# NSGA-II Parameters (can be overridden in main section)
POPULATION_SIZE = 100
MAX_GENERATIONS = 30
CROSSOVER_PROB = 0.9
# MUTATION_PROB will be set after NUM_UAVS is known

# --- Global Variables for NSGA-II using Greedy Coverage ---
# These will be populated after the Greedy run
ALL_GREED_NODES_GLOBAL = []
PLOT_AREA_GLOBAL = {}
MAX_POSSIBLE_AREA_GLOBAL = 0.0
GRID_DENSITY_GLOBAL = DEFAULT_GRID_DENSITY_GREEDY
NSGA2_COVERAGE_TARGET = DEFAULT_TARGET_COVERAGE # Now interpreted as ratio of MAX_POSSIBLE_AREA_GLOBAL


# =============================================================================
# SECTION 1: Code from Greed2.py (Classes and Functions)
# =============================================================================

# --- GPS Conversion Class ---
class GPSCoordPlotter:
    # (Keep the class definition from Greed2.py exactly as it is)
    def __init__(self, master_gps):
        if not isinstance(master_gps, (tuple, list)) or len(master_gps) != 2:
            raise ValueError("Master GPS must be a tuple or list of (latitude, longitude)")
        self.master_lat, self.master_lon = master_gps
        self.earth_radius = 6371000  # Meters

    def _haversine(self, lat1, lon1, lat2, lon2):
        # (Keep the _haversine method)
        phi1, phi2 = math.radians(lat1), math.radians(lat2)
        delta_phi = math.radians(lat2 - lat1)
        delta_lambda = math.radians(lon2 - lon1)
        a = math.sin(delta_phi / 2.0)**2 + math.cos(phi1) * math.cos(phi2) * math.sin(delta_lambda / 2.0)**2
        c = 2 * math.atan2(math.sqrt(a), math.sqrt(1.0 - a))
        return self.earth_radius * c

    def gps_to_xy(self, target_lat, target_lon):
        # (Keep the gps_to_xy method)
        y_dist = self._haversine(self.master_lat, self.master_lon, target_lat, self.master_lon)
        if target_lat < self.master_lat: y_dist *= -1
        x_dist = self._haversine(self.master_lat, self.master_lon, self.master_lat, target_lon)
        if target_lon < self.master_lon: x_dist *= -1
        return {'x': x_dist, 'y': y_dist}

    def convert_nodes_to_xy(self, drone_nodes_data):
        # (Keep the convert_nodes_to_xy method)
        node_coords = {}
        all_x = [0.0]; all_y = [0.0]
        valid_nodes_count = 0
        for node in drone_nodes_data:
            node_id = node.get('id')
            gps = node.get('gps')
            if node_id and gps and isinstance(gps, (tuple, list)) and len(gps) == 2:
                try:
                    coords = self.gps_to_xy(gps[0], gps[1])
                    node_coords[node_id] = coords
                    all_x.append(coords['x'])
                    all_y.append(coords['y'])
                    valid_nodes_count += 1
                except Exception as e:
                    print(f"Warning: Could not convert GPS for node {node_id}: {e}")
            else:
                print(f"Warning: Skipping node due to missing/invalid ID or GPS: {node}")

        if valid_nodes_count == 0:
             print("Warning: No valid node GPS coordinates found to convert.")
             plot_area = {'xlim': (-100, 100), 'ylim': (-100, 100)}
             return {}, plot_area

        if not all_x or not all_y:
            plot_area = {'xlim': (-100, 100), 'ylim': (-100, 100)}
        else:
            min_x, max_x = min(all_x), max(all_x)
            min_y, max_y = min(all_y), max(all_y)
            margin_x = (max_x - min_x) * 0.15 if max_x > min_x else 50
            margin_y = (max_y - min_y) * 0.15 if max_y > min_y else 50
            plot_area = {
                'xlim': (min_x - margin_x, max_x + margin_x),
                'ylim': (min_y - margin_y, max_y + margin_y)
            }
        print(f"GPS to XY conversion complete for {valid_nodes_count} nodes.")
        return node_coords, plot_area

# --- Node Scoring Class ---
class NodeScorer:
    # (Keep the class definition from Greed2.py exactly as it is)
    def __init__(self, drone_nodes_data):
        self.nodes = drone_nodes_data
        self.max_battery = 100; self.min_battery = 0
        self.max_latency = 400; self.min_latency = 0
        self.max_util = 100; self.min_util = 0

    def _normalize(self, value, min_val, max_val):
        if max_val == min_val: return 0.5
        return max(0.0, min(1.0, (value - min_val) / (max_val - min_val)))

    def calculate_scores(self, task_type='default'):
        # (Keep the calculate_scores method)
        scores = []
        weights = {}
        if task_type == 'emergency': weights = {'battery': 0.4, 'latency': 0.5, 'util': 0.1}
        elif task_type == 'sustain': weights = {'battery': 0.5, 'latency': 0.2, 'util': 0.3}
        elif task_type == 'compute': weights = {'battery': 0.2, 'latency': 0.4, 'util': 0.4}
        else: weights = {'battery': 0.33, 'latency': 0.34, 'util': 0.33}
        print(f"Using scoring weights for task '{task_type}': {weights}")
        for node in self.nodes:
            node_id = node.get('id')
            if not node_id: continue
            battery = node.get('battery', 0)
            latency = node.get('latency', self.max_latency)
            util = node.get('util', self.max_util)
            norm_battery = self._normalize(battery, self.min_battery, self.max_battery)
            norm_latency_inv = 1.0 - self._normalize(latency, self.min_latency, self.max_latency)
            norm_util_inv = 1.0 - self._normalize(util, self.min_util, self.max_util)
            total_score = (norm_battery * weights.get('battery', 0) +
                           norm_latency_inv * weights.get('latency', 0) +
                           norm_util_inv * weights.get('util', 0))
            final_score = max(0.001, total_score) if total_score > 0 else 0
            scores.append({'id': node_id, 'total': final_score})
        return scores

# --- Greed Node Class ---
class GreedNode:
    # (Keep the class definition from Greed2.py exactly as it is)
    def __init__(self, id, x, y, radius, score):
        self.id = id
        self.x = float(x) if isinstance(x, (int, float, np.number)) else 0.0
        self.y = float(y) if isinstance(y, (int, float, np.number)) else 0.0
        self.radius = float(radius) if isinstance(radius, (int, float, np.number)) and radius > 0 else 0.0
        self.score = float(score) if isinstance(score, (int, float, np.number)) and score > 0 else 0.0
        self.area = np.pi * self.radius**2 if self.radius > 0 else 0.0
    def __repr__(self):
        return (f"GreedNode(id={self.id}, pos=({self.x:.2f},{self.y:.2f}), "
                f"r={self.radius:.2f}, area={self.area:.2f}, score={self.score:.2f})")

# --- Greed Helper Functions ---
def calculate_union_area(nodes, plot_area, grid_density=50):
    # (Keep the function definition from Greed2.py exactly as it is)
    # THIS IS NOW THE PRIMARY COVERAGE CALCULATION METHOD FOR BOTH ALGORITHMS
    if not nodes: return 0.0
    valid_nodes = [node for node in nodes if hasattr(node, 'radius') and node.radius > 0] # Check attribute existence
    if not valid_nodes: return 0.0
    if not plot_area or 'xlim' not in plot_area or 'ylim' not in plot_area or not plot_area['xlim'] or not plot_area['ylim']:
         print("Error: calculate_union_area needs valid plot_area with non-empty limits.")
         return 0.0

    xlim = plot_area['xlim']
    ylim = plot_area['ylim']
    # Add check for valid range
    if xlim[1] <= xlim[0] or ylim[1] <= ylim[0]:
         print(f"Warning: Invalid plot_area range in calculate_union_area. xlim={xlim}, ylim={ylim}. Returning 0 area.")
         return 0.0

    x_range = xlim[1] - xlim[0]
    y_range = ylim[1] - ylim[0]

    # Ensure density is reasonable
    safe_grid_density = max(1, int(grid_density))

    grid_size_x = x_range / safe_grid_density
    grid_size_y = y_range / safe_grid_density
    grid_cell_area = grid_size_x * grid_size_y

    # Adjust linspace num to be integer
    x_points = np.linspace(xlim[0] + grid_size_x / 2, xlim[1] - grid_size_x / 2, safe_grid_density)
    y_points = np.linspace(ylim[0] + grid_size_y / 2, ylim[1] - grid_size_y / 2, safe_grid_density)
    if x_points.size == 0 or y_points.size == 0: return 0.0

    xx, yy = np.meshgrid(x_points, y_points)
    grid_points = np.column_stack([xx.ravel(), yy.ravel()])
    if grid_points.size == 0: return 0.0

    covered_mask = np.zeros(len(grid_points), dtype=bool)
    # Ensure nodes have x, y attributes before creating array
    node_centers = np.array([[node.x, node.y] for node in valid_nodes if hasattr(node, 'x') and hasattr(node, 'y')])
    node_radii = np.array([node.radius for node in valid_nodes]) # Assuming radius check already done
    if node_centers.size == 0: return 0.0

    # Check array dimensions before cdist
    if grid_points.ndim != 2 or node_centers.ndim != 2 or grid_points.shape[1] != node_centers.shape[1]:
         print("Error: Dimension mismatch for cdist in calculate_union_area.")
         print(f"grid_points shape: {grid_points.shape}")
         print(f"node_centers shape: {node_centers.shape}")
         return 0.0 # Cannot calculate distance

    distances_to_centers = cdist(grid_points, node_centers) # Shape: (n_grid_points, n_valid_nodes)
    # Ensure radii array matches the number of columns in distances_to_centers
    if distances_to_centers.shape[1] != len(node_radii):
         print(f"Error: Mismatch between distance columns ({distances_to_centers.shape[1]}) and number of radii ({len(node_radii)}).")
         return 0.0
    covered_mask = np.any(distances_to_centers <= node_radii, axis=1)
    return np.sum(covered_mask) * grid_cell_area


def greedy_node_selection_by_coverage(all_greed_nodes, target_coverage_ratio, plot_area, grid_density=50):
    # (Keep the function definition from Greed2.py exactly as it is)
    # Calculates and returns max_possible_union_area needed by NSGA-II
    print("\n--- Greedy Algorithm Selection (Coverage Target) ---")
    start_time = time.time()

    selected_nodes = []
    candidate_nodes = [node for node in all_greed_nodes if node.area > 0 and node.score > 0]
    remaining_nodes = list(candidate_nodes)

    if not candidate_nodes:
        print("No valid (area>0, score>0) candidates for Greedy selection.")
        return [], 0.0, 0.0, 0.0 # Added return for max_possible_area

    print("Calculating max possible coverage area for candidates...")
    # *** Crucial calculation for NSGA-II target ***
    max_possible_union_area = calculate_union_area(candidate_nodes, plot_area, grid_density)

    if max_possible_union_area <= 1e-6:
        print("Warning: Max possible coverage area estimated close to zero. Cannot use coverage ratio target.")
        if target_coverage_ratio > 0 and remaining_nodes:
             best_fallback_node = max(remaining_nodes, key=lambda n: n.score * n.area)
             print(f"Fallback: Selecting single best node by score*area: {best_fallback_node.id}")
             fallback_area = calculate_union_area([best_fallback_node], plot_area, grid_density)
             return [best_fallback_node], fallback_area, 1.0, max_possible_union_area # Achieved 100% of (near) zero max
        else:
             return [], 0.0, 0.0, max_possible_union_area
    else:
        print(f"Max Possible Coverage Area (Est.): {max_possible_union_area:.2f} m² (from {len(candidate_nodes)} candidates)")

    print(f"Target Coverage Ratio: {target_coverage_ratio * 100:.1f}%")
    print("-" * 30)

    current_union_area = 0.0
    achieved_coverage_ratio = 0.0
    iteration = 0

    while achieved_coverage_ratio < target_coverage_ratio and remaining_nodes:
        iteration += 1
        best_node_to_add = None
        best_gain = -1.0
        best_incremental_area = 0.0

        for node in remaining_nodes:
            temp_selection = selected_nodes + [node]
            # Use the global plot_area and grid_density consistent with NSGA-II
            new_total_union_area = calculate_union_area(temp_selection, plot_area, grid_density)
            incremental_area = max(0.0, new_total_union_area - current_union_area)
            gain = incremental_area * node.score

            if gain > best_gain:
                best_gain = gain; best_node_to_add = node; best_incremental_area = incremental_area
            elif gain == best_gain and node.score > (best_node_to_add.score if best_node_to_add else -1):
                 best_node_to_add = node; best_incremental_area = incremental_area

        if best_node_to_add is None:
            print(f"Round {iteration}: No more nodes to select."); break
        if best_gain <= 0 and len(selected_nodes) > 0:
             print(f"Round {iteration}: No node provides positive gain. Stopping."); break

        print(f"--- Round {iteration} Selection ---")
        print(f"Selected Node {best_node_to_add.id} (Score: {best_node_to_add.score:.2f}, Radius: {best_node_to_add.radius:.1f}m)")
        print(f"  Incremental Area (Est): {best_incremental_area:.2f} m²")
        print(f"  Selection Gain (Inc. Area * Score): {best_gain:.2f}")
        selected_nodes.append(best_node_to_add)
        remaining_nodes.remove(best_node_to_add)

        current_union_area = calculate_union_area(selected_nodes, plot_area, grid_density) # Recalculate
        achieved_coverage_ratio = current_union_area / max_possible_union_area

        print(f"  New Total Area (Est): {current_union_area:.2f} m²")
        print(f"  Current Coverage Ratio: {achieved_coverage_ratio * 100:.1f}% / {target_coverage_ratio * 100:.1f}%")
        print("-" * 20)

        if achieved_coverage_ratio >= target_coverage_ratio:
            print(f"Target coverage ratio reached or exceeded. Stopping."); break

    end_time = time.time()
    print(f"\nGreedy selection finished. Time: {end_time - start_time:.2f}s")

    final_union_area = calculate_union_area(selected_nodes, plot_area, grid_density)
    final_achieved_ratio = final_union_area / max_possible_union_area if max_possible_union_area > 1e-6 else (1.0 if final_union_area > 1e-6 else 0.0)

    # Return max_possible_union_area as well
    return selected_nodes, final_union_area, final_achieved_ratio, max_possible_union_area


def visualize_greed_selection(all_nodes, selected_nodes, master_coord_xy, plot_area, grid_density, title_info="Greedy Selection"):
    # (Keep the visualization function from Greed2.py)
    # Pass grid_density for accurate coverage % calculation in title
    fig, ax = plt.subplots(figsize=(10, 10))

    if plot_area and 'xlim' in plot_area and 'ylim' in plot_area:
        ax.set_xlim(plot_area['xlim']); ax.set_ylim(plot_area['ylim'])
    else:
        ax.set_xlim(-500, 500); ax.set_ylim(-500, 500)
        print("Warning: Plot area missing, using default view.")
    ax.set_aspect('equal', adjustable='box')

    selected_ids = {node.id for node in selected_nodes}
    unselected_nodes = [node for node in all_nodes if node.id not in selected_ids]

    for node in unselected_nodes: # Unselected
        if node.radius > 0:
            circle = Circle((node.x, node.y), node.radius, edgecolor='gray', facecolor='lightblue', alpha=0.3, linewidth=0.5)
            ax.add_patch(circle)
    for node in selected_nodes: # Selected
         if node.radius > 0:
            circle = Circle((node.x, node.y), node.radius, edgecolor='black', facecolor='red', alpha=0.6, linewidth=1)
            ax.add_patch(circle)
            ax.text(node.x, node.y, node.id, ha='center', va='center', fontsize=7, weight='bold', color='white')

    ax.plot(master_coord_xy[0], master_coord_xy[1], 'P', markersize=10, color='gold', markeredgecolor='black', label='Master Node', zorder=10)

    # Recalculate coverage based on passed density for title accuracy
    final_union_area = calculate_union_area(selected_nodes, plot_area, grid_density)
    valid_candidate_nodes = [n for n in all_nodes if n.area > 0 and n.score > 0]
    all_valid_union_area = calculate_union_area(valid_candidate_nodes, plot_area, grid_density)
    coverage_percent = (final_union_area / all_valid_union_area * 100) if all_valid_union_area > 1e-6 else (100 if final_union_area > 1e-6 else 0)
    total_initial_valid_nodes = len([n for n in all_nodes if n.area > 0])

    plt.title(f"{title_info}\n"
              f"Selected: {len(selected_nodes)}/{len(valid_candidate_nodes)} candidates. "
              f"Est. Area: {final_union_area:.2f} m². Achieved Cov Ratio: {coverage_percent:.1f}%",
              fontsize=10)
    plt.xlabel("East-West Relative Distance (m)"); plt.ylabel("North-South Relative Distance (m)")
    plt.grid(True, linestyle='--', alpha=0.5); plt.legend(); plt.tight_layout()
    # plt.show() # Control showing from main


# --- Greed Workflow Function ---
def run_greedy_workflow(master_node_gps, drone_nodes_data, task_type, target_coverage_ratio, grid_density=50, show_plot=False):
    # (Keep the workflow function from Greed2.py, but make plotting optional and return results)
    # Returns the necessary info for NSGA-II coverage calcs
    print("="*50); print("Starting Greedy Workflow")
    print(f"Task: {task_type}, Target Coverage Ratio: {target_coverage_ratio*100:.1f}%")
    print("="*50)
    workflow_start_time = time.time()

    # 1. GPS Conversion
    print("\nStep 1: Converting GPS to XY...")
    gps_converter = GPSCoordPlotter(master_node_gps)
    plot_area = None; node_coords = {}
    try:
        node_coords, plot_area = gps_converter.convert_nodes_to_xy(drone_nodes_data)
        if not node_coords: print("Error: No valid node coords. Aborting Greedy."); return None
    except Exception as e: print(f"Error during GPS conversion: {e}"); return None
    print(f"Plot area for Greedy: xlim={plot_area.get('xlim')}, ylim={plot_area.get('ylim')}")

    # 2. Node Scoring
    print("\nStep 2: Scoring Nodes...")
    scorer = NodeScorer(drone_nodes_data)
    node_scores = []; score_map = {}
    try:
        node_scores = scorer.calculate_scores(task_type)
        if not node_scores: print("Warning: No node scores calculated.")
        score_map = {score['id']: score['total'] for score in node_scores}
    except Exception as e: print(f"Error during node scoring: {e}"); return None

    # 3. Prepare GreedNodes
    print("\nStep 3: Preparing GreedNodes...")
    all_greed_nodes = []; node_order_ids = []
    for node_data in drone_nodes_data:
        node_id = node_data.get('id')
        if not node_id: continue
        node_order_ids.append(node_id)
        coords = node_coords.get(node_id)
        score = score_map.get(node_id, 0)
        radius = node_data.get('radius')
        if coords and radius is not None:
             try:
                 greed_node = GreedNode(id=node_id, x=coords['x'], y=coords['y'], radius=radius, score=score)
                 all_greed_nodes.append(greed_node)
             except Exception as e: print(f"Error creating GreedNode for {node_id}: {e}")

    valid_candidate_count = sum(1 for node in all_greed_nodes if node.area > 0 and node.score > 0)
    print(f"Prepared {len(all_greed_nodes)} GreedNodes ({valid_candidate_count} valid candidates).")
    if valid_candidate_count == 0: print("Error: No valid candidates. Aborting Greedy."); return None

    # 4. Run Greedy Selection
    print("\nStep 4: Running Greedy Selection Algorithm...")
    selected_greed_nodes, final_union_area, achieved_coverage_ratio, max_possible_area = greedy_node_selection_by_coverage(
        all_greed_nodes, target_coverage_ratio, plot_area, grid_density
    )

    # 5. Generate Deployment Indicator
    selected_ids = {node.id for node in selected_greed_nodes}
    deployment_indicator = [1 if node_id in selected_ids else 0 for node_id in node_order_ids]

    # 6. Print Summary
    print("\n" + "="*50); print("Greedy Workflow Summary"); print("="*50)
    print(f"Selected Count: {len(selected_greed_nodes)}")
    print(f"Final Area (Est): {final_union_area:.2f} m²")
    if achieved_coverage_ratio is not None:
        print(f"Achieved Coverage Ratio: {achieved_coverage_ratio * 100:.1f}% (Target: {target_coverage_ratio*100:.1f}%)")
    print(f"Deployment Indicator: {deployment_indicator}")
    workflow_end_time = time.time()
    print(f"\nGreedy Workflow Time: {workflow_end_time - workflow_start_time:.2f}s"); print("="*50)

    # 7. Visualization (Optional)
    if show_plot:
        print("\nStep 5: Visualizing Greedy Results...")
        try:
            master_xy = (0.0, 0.0)
            title = f"Greedy (Task: {task_type}, Target Cov Ratio: {target_coverage_ratio*100:.1f}%)"
            # Pass grid_density for accurate title calculation
            visualize_greed_selection(all_greed_nodes, selected_greed_nodes, master_xy, plot_area, grid_density, title_info=title)
            plt.show() # Show the plot now
            print("Greedy visualization generated.")
        except Exception as e:
            print(f"Error during Greedy visualization: {e}")

    # Return results including data needed for NSGA-II coverage
    return {
        'selected_nodes_greedy': selected_greed_nodes,
        'final_area_greedy': final_union_area,
        'achieved_coverage_greedy': achieved_coverage_ratio,
        'deployment_indicator': deployment_indicator,
        'node_order_ids': node_order_ids,
        'all_greed_nodes': all_greed_nodes, # <- Needed by NSGA-II
        'plot_area_greedy': plot_area,       # <- Needed by NSGA-II
        'max_possible_area': max_possible_area, # <- Needed by NSGA-II
        'grid_density_used': grid_density   # <- Needed by NSGA-II
    }


# =============================================================================
# SECTION 2: NSGA-II Specific Code (Using Greedy Coverage)
# =============================================================================

# --- REMOVED NSGA-II GRID HELPER FUNCTIONS ---
# nsga2_haversine (can be kept if needed elsewhere, but not for coverage)
# gps_to_grid (REMOVED)
# meters_to_grid_radius (REMOVED)
# calculate_nsga2_coverage (REMOVED)

# --- Global NSGA-II Data Structures (initialized in main section) ---
NUM_UAVS = 0
UAV_DATA = {} # Dict: {'battery': [], 'delay': [], 'utilization': []} - Still needed for objectives

# --- NSGA-II Objective and Constraint Functions (Using Greedy Coverage) ---

def calculate_objectives(individual_chromosome):
    # (Keep the function from NSGA2.py, using UAV_DATA)
    # Assumes UAV_DATA is populated correctly with 'battery', 'delay', 'utilization'
    selected_indices = [i for i, selected in enumerate(individual_chromosome) if selected == 1]
    num_selected = len(selected_indices)

    if num_selected == 0:
        return [-0.0, float('inf'), float('inf'), 0]

    valid_indices = [idx for idx in selected_indices if 0 <= idx < NUM_UAVS]
    if not valid_indices:
        print("Warning: No valid indices in chromosome for objective calculation.")
        return [-0.0, float('inf'), float('inf'), 0]
    num_selected = len(valid_indices)

    if not UAV_DATA or not all(k in UAV_DATA for k in ['battery', 'delay', 'utilization']) or \
       len(UAV_DATA['battery']) != NUM_UAVS:
         print("Error: UAV_DATA not correctly populated for objective calculation.")
         return [-0.0, float('inf'), float('inf'), num_selected]

    # Ensure indices are valid for UAV_DATA access
    try:
        avg_battery = np.sum(UAV_DATA['battery'][valid_indices]) / num_selected
        avg_delay = np.sum(UAV_DATA['delay'][valid_indices]) / num_selected
        avg_utilization = np.sum(UAV_DATA['utilization'][valid_indices]) / num_selected
        num_uavs = num_selected
    except IndexError:
         print(f"Error: Invalid index encountered during objective calculation. Max index: {NUM_UAVS-1}, Indices: {valid_indices}")
         return [-0.0, float('inf'), float('inf'), num_selected] # Penalize

    objectives = [
        -avg_battery,      # Minimize negative avg battery
        avg_delay,         # Minimize avg delay
        avg_utilization,   # Minimize avg utilization
        num_uavs           # Minimize num UAVs
    ]
    return objectives

def check_constraints(individual_chromosome):
    """ Checks constraints using the Greedy coverage calculation method. """
    global ALL_GREED_NODES_GLOBAL, PLOT_AREA_GLOBAL, MAX_POSSIBLE_AREA_GLOBAL, GRID_DENSITY_GLOBAL, NSGA2_COVERAGE_TARGET

    # Check if necessary global data is available
    if not ALL_GREED_NODES_GLOBAL or not PLOT_AREA_GLOBAL or MAX_POSSIBLE_AREA_GLOBAL is None:
        print("Error: Missing global data needed for NSGA-II constraint check. Assuming infeasible.")
        return False

    selected_indices = [i for i, selected in enumerate(individual_chromosome) if selected == 1]

    # Handle no selection
    if not selected_indices:
        return False if NSGA2_COVERAGE_TARGET > 0 else True

    # Map indices to GreedNode objects
    selected_greed_nodes = []
    for idx in selected_indices:
        if 0 <= idx < len(ALL_GREED_NODES_GLOBAL):
            selected_greed_nodes.append(ALL_GREED_NODES_GLOBAL[idx])
        else:
            print(f"Warning: Invalid index {idx} in chromosome during constraint check.")
            # Optionally return False immediately or just skip the node
            return False # Safer to assume infeasible if index is bad

    if not selected_greed_nodes:
        return False if NSGA2_COVERAGE_TARGET > 0 else True # No valid nodes selected

    # Calculate coverage using Greedy's method
    current_union_area = calculate_union_area(
        selected_greed_nodes,
        PLOT_AREA_GLOBAL,
        GRID_DENSITY_GLOBAL
    )

    # Constraint 1: Coverage Ratio >= TARGET
    if MAX_POSSIBLE_AREA_GLOBAL <= 1e-6:
        # If max possible area is near zero, any non-zero coverage meets the target ratio conceptually
        coverage_ratio = 1.0 if current_union_area > 1e-6 else 0.0
    else:
        coverage_ratio = current_union_area / MAX_POSSIBLE_AREA_GLOBAL

    if coverage_ratio < NSGA2_COVERAGE_TARGET:
        return False # Violated

    # Add other constraints if needed...

    return True # All constraints met

# --- NSGA-II Repair Mechanism (Using Greedy Coverage) ---
def repair_individual(individual_chromosome):
    """ Attempts to repair an individual to meet the coverage constraint using Greedy's method. """
    global ALL_GREED_NODES_GLOBAL, PLOT_AREA_GLOBAL, MAX_POSSIBLE_AREA_GLOBAL, GRID_DENSITY_GLOBAL, NSGA2_COVERAGE_TARGET, UAV_DATA

    # Check if repair is needed
    if check_constraints(individual_chromosome):
        return individual_chromosome

    # Check if necessary global data is available
    if not ALL_GREED_NODES_GLOBAL or not PLOT_AREA_GLOBAL or MAX_POSSIBLE_AREA_GLOBAL is None or not UAV_DATA:
        print("Error: Missing global data needed for NSGA-II repair. Returning original chromosome.")
        return list(individual_chromosome) # Return copy

    repaired_chromosome = list(individual_chromosome)
    selected_indices = [i for i, selected in enumerate(repaired_chromosome) if selected == 1]

    # Map current selection to GreedNodes
    current_selected_nodes = [ALL_GREED_NODES_GLOBAL[idx] for idx in selected_indices if 0 <= idx < len(ALL_GREED_NODES_GLOBAL)]

    # Calculate current state using Greedy method
    current_union_area = calculate_union_area(current_selected_nodes, PLOT_AREA_GLOBAL, GRID_DENSITY_GLOBAL)
    current_coverage_ratio = current_union_area / MAX_POSSIBLE_AREA_GLOBAL if MAX_POSSIBLE_AREA_GLOBAL > 1e-6 else (1.0 if current_union_area > 1e-6 else 0.0)

    # Identify candidates to add
    unselected_indices = [i for i, selected in enumerate(repaired_chromosome) if selected == 0]
    if not unselected_indices:
        # print("Warning: Repair impossible - no unselected UAVs left.")
        return repaired_chromosome # Cannot repair

    potential_additions = []
    for idx in unselected_indices:
         if 0 <= idx < len(ALL_GREED_NODES_GLOBAL):
            node_to_consider = ALL_GREED_NODES_GLOBAL[idx]
            # Calculate coverage with the potential node added
            temp_selection = current_selected_nodes + [node_to_consider]
            new_total_union_area = calculate_union_area(temp_selection, PLOT_AREA_GLOBAL, GRID_DENSITY_GLOBAL)
            # Calculate gain in AREA (not necessarily ratio)
            incremental_area_gain = max(0.0, new_total_union_area - current_union_area)
            # Use score from the GreedNode for gain calculation
            score = node_to_consider.score
            gain = incremental_area_gain * score
            # Get delay for tie-breaking
            delay = UAV_DATA['delay'][idx] if UAV_DATA and idx < len(UAV_DATA.get('delay', [])) else float('inf')

            potential_additions.append({'gain': gain, 'index': idx, 'delay': delay, 'inc_area': incremental_area_gain})
         # else: Invalid index, ignore

    # Sort by gain (desc), then by delay (asc)
    potential_additions.sort(key=lambda x: (x['gain'], -x['delay']), reverse=True)

    # Greedily add nodes
    for item in potential_additions:
        if current_coverage_ratio >= NSGA2_COVERAGE_TARGET:
            break # Stop if target met

        idx_to_add = item['index']
        # Add the node if it wasn't selected before
        if 0 <= idx_to_add < NUM_UAVS and repaired_chromosome[idx_to_add] == 0:
             repaired_chromosome[idx_to_add] = 1
             # Update the list of selected nodes for the next iteration's area calculation
             current_selected_nodes.append(ALL_GREED_NODES_GLOBAL[idx_to_add])
             # Update area and ratio (more accurate to recalculate fully)
             current_union_area = calculate_union_area(current_selected_nodes, PLOT_AREA_GLOBAL, GRID_DENSITY_GLOBAL)
             current_coverage_ratio = current_union_area / MAX_POSSIBLE_AREA_GLOBAL if MAX_POSSIBLE_AREA_GLOBAL > 1e-6 else (1.0 if current_union_area > 1e-6 else 0.0)
             # print(f"  Repair: Added UAV {idx_to_add}, New Area: {current_union_area:.2f}, New Ratio: {current_coverage_ratio:.3f}")


    # Final check (optional logging)
    # final_check_ratio = calculate_union_area([ALL_GREED_NODES_GLOBAL[i] for i, sel in enumerate(repaired_chromosome) if sel == 1], PLOT_AREA_GLOBAL, GRID_DENSITY_GLOBAL) / MAX_POSSIBLE_AREA_GLOBAL
    # if final_check_ratio < NSGA2_COVERAGE_TARGET:
    #     print(f"Warning: Repair finished but ratio ({final_check_ratio:.3f}) still below target ({NSGA2_COVERAGE_TARGET:.3f}).")

    # Ensure not empty if repair accidentally removed all
    if sum(repaired_chromosome) == 0 and NUM_UAVS > 0:
        random_idx = random.randrange(NUM_UAVS)
        repaired_chromosome[random_idx] = 1
        print("Warning: Repair resulted in empty chromosome, added a random UAV.")

    return repaired_chromosome


# --- NSGA-II Individual Class ---
class Individual:
    # (Keep the class definition from NSGA2.py)
    # Constructor and calculate_fitness use the modified constraint/repair funcs implicitly
    def __init__(self, chromosome=None):
        global NUM_UAVS
        if chromosome is not None:
             if len(chromosome) == NUM_UAVS:
                 self.chromosome = list(chromosome)
             else:
                 print(f"Warning: Provided chromosome length ({len(chromosome)}) != NUM_UAVS ({NUM_UAVS}). Generating random.")
                 self.chromosome = self._generate_potentially_feasible_chromosome()
        else:
            self.chromosome = self._generate_potentially_feasible_chromosome()

        self.objectives = []
        self.is_feasible = False
        self.rank = float('inf')
        self.crowding_distance = 0.0
        self.dominated_solutions = set()
        self.domination_count = 0

    def _generate_potentially_feasible_chromosome(self):
        global NUM_UAVS
        attempts = 0
        max_attempts = max(10, NUM_UAVS // 5) if NUM_UAVS > 0 else 10
        # Handle case where NUM_UAVS might be 0 initially
        if NUM_UAVS <= 0: return []

        while attempts < max_attempts:
            num_to_select = random.randint(1, max(1, int(NUM_UAVS * 0.2))) # Ensure at least 1 if possible
            # Ensure sample size isn't larger than population
            if num_to_select > NUM_UAVS: num_to_select = NUM_UAVS
            if NUM_UAVS > 0 :
                indices_to_select = random.sample(range(NUM_UAVS), num_to_select)
                chromosome = [1 if i in indices_to_select else 0 for i in range(NUM_UAVS)]
            else:
                chromosome = [] # Empty if no UAVs

            repaired = repair_individual(chromosome) # Uses new repair logic
            # Check feasibility AFTER repair
            if check_constraints(repaired): # Uses new check logic
                return repaired
            attempts += 1

        # Fallback
        chromosome = [random.randint(0, 1) for _ in range(NUM_UAVS)]
        if sum(chromosome) == 0 and NUM_UAVS > 0: chromosome[random.randrange(NUM_UAVS)] = 1
        elif NUM_UAVS == 0: chromosome = []
        return repair_individual(chromosome)


    def calculate_fitness(self):
        # Uses new check_constraints implicitly
        self.objectives = calculate_objectives(self.chromosome)
        self.is_feasible = check_constraints(self.chromosome)

    def dominates(self, other_individual):
        # (Keep this method using constrained dominance)
        # It relies on the self.is_feasible flag which is now set by the new check_constraints
        if self.is_feasible and not other_individual.is_feasible: return True
        if not self.is_feasible and other_individual.is_feasible: return False
        if self.is_feasible and other_individual.is_feasible:
            better_in_any = False
            if not self.objectives: self.calculate_fitness()
            if not other_individual.objectives: other_individual.calculate_fitness()
            if not self.objectives or not other_individual.objectives: return False
            # Check objectives length match before iterating
            if len(self.objectives) != len(other_individual.objectives): return False

            for i in range(len(self.objectives)):
                 self_obj = self.objectives[i]; other_obj = other_individual.objectives[i]
                 # Robust infinity checks
                 is_self_inf = isinstance(self_obj, float) and math.isinf(self_obj)
                 is_other_inf = isinstance(other_obj, float) and math.isinf(other_obj)
                 if is_self_inf and not is_other_inf: return False # self is infinitely bad
                 if not is_self_inf and is_other_inf: better_in_any = True; continue # self is better
                 if is_self_inf and is_other_inf: continue # Both infinitely bad

                 # Ensure they are comparable numbers before comparison
                 if not isinstance(self_obj, (int, float)) or not isinstance(other_obj, (int, float)):
                     return False # Cannot compare non-numeric

                 if self_obj > other_obj: return False
                 if self_obj < other_obj: better_in_any = True
            return better_in_any
        return False


# --- NSGA-II Core Components ---
# (fast_non_dominated_sort, calculate_crowding_distance, binary_tournament_selection,
#  crossover, mutate remain the same as they operate on chromosomes/ranks/distances,
#  not directly on coverage calculation)

def fast_non_dominated_sort(population):
    # (Keep the function from NSGA2.py - No changes needed)
    fronts = [[]]; population_size = len(population)
    for p_idx in range(population_size):
        p = population[p_idx]
        p.dominated_solutions = set(); p.domination_count = 0
        if not p.objectives: p.calculate_fitness() # Ensures is_feasible is set via new check_constraints
        for q_idx in range(population_size):
            if p_idx == q_idx: continue
            q = population[q_idx]
            if not q.objectives: q.calculate_fitness()
            if p.dominates(q): p.dominated_solutions.add(q_idx)
            elif q.dominates(p): p.domination_count += 1
        if p.domination_count == 0: p.rank = 0; fronts[0].append(p_idx)
    i = 0
    while i < len(fronts) and fronts[i]:
        next_front_indices = []
        for p_idx in fronts[i]:
            p = population[p_idx]
            for q_idx in p.dominated_solutions:
                q = population[q_idx]
                q.domination_count -= 1
                if q.domination_count == 0: q.rank = i + 1; next_front_indices.append(q_idx)
        i += 1
        if next_front_indices: fronts.append(next_front_indices)
        else: break
    return fronts

def calculate_crowding_distance(front_indices, population):
    # (Keep the function from NSGA2.py - No changes needed)
    if not front_indices: return
    num_individuals = len(front_indices)
    if num_individuals == 0: return
    first_ind_idx = front_indices[0]
    if not population[first_ind_idx].objectives:
        for idx in front_indices: population[idx].crowding_distance = 0.0
        return
    num_objectives = len(population[first_ind_idx].objectives)
    for idx in front_indices: population[idx].crowding_distance = 0.0
    for m in range(num_objectives):
        try:
            sorted_indices = sorted(
                front_indices,
                key=lambda i: population[i].objectives[m] if (population[i].objectives and
                                                               len(population[i].objectives) > m and
                                                               isinstance(population[i].objectives[m], (int, float)) and
                                                               not math.isinf(population[i].objectives[m]) and
                                                               not math.isnan(population[i].objectives[m])) else float('inf')
            )
        except Exception as e: continue # Skip obj if sort fails
        if not sorted_indices: continue
        f_min_obj = population[sorted_indices[0]].objectives[m]
        f_max_obj = population[sorted_indices[-1]].objectives[m]
        population[sorted_indices[0]].crowding_distance = float('inf')
        if num_individuals > 1:
            population[sorted_indices[-1]].crowding_distance = float('inf')
            is_fmin_valid = isinstance(f_min_obj, (int, float)) and not math.isinf(f_min_obj) and not math.isnan(f_min_obj)
            is_fmax_valid = isinstance(f_max_obj, (int, float)) and not math.isinf(f_max_obj) and not math.isnan(f_max_obj)
            if is_fmin_valid and is_fmax_valid: obj_range = f_max_obj - f_min_obj
            else: obj_range = 0
        else: obj_range = 0
        if obj_range is None or obj_range <= 0 or math.isinf(obj_range) or math.isnan(obj_range): continue
        for i in range(1, num_individuals - 1):
            curr_idx = sorted_indices[i]; prev_idx = sorted_indices[i-1]; next_idx = sorted_indices[i+1]
            obj_next = population[next_idx].objectives[m]; obj_prev = population[prev_idx].objectives[m]
            is_next_valid = isinstance(obj_next, (int, float)) and not math.isinf(obj_next) and not math.isnan(obj_next)
            is_prev_valid = isinstance(obj_prev, (int, float)) and not math.isinf(obj_prev) and not math.isnan(obj_prev)
            if is_next_valid and is_prev_valid:
                 dist_contrib = (obj_next - obj_prev) / obj_range
                 if population[curr_idx].crowding_distance != float('inf'):
                     population[curr_idx].crowding_distance += dist_contrib

def binary_tournament_selection(population):
    # (Keep the function from NSGA2.py - No changes needed)
    pop_size = len(population)
    if pop_size == 0: raise ValueError("Empty population for selection")
    if pop_size == 1: return population[0]
    p1_idx, p2_idx = random.sample(range(pop_size), 2)
    p1 = population[p1_idx]; p2 = population[p2_idx]
    if p1.rank < p2.rank: return p1
    elif p2.rank < p1.rank: return p2
    elif p1.crowding_distance > p2.crowding_distance: return p1
    elif p2.crowding_distance > p1.crowding_distance: return p2
    else: return random.choice([p1, p2])

def crossover(parent1, parent2):
    # (Keep the function from NSGA2.py - Uniform Crossover)
    global NUM_UAVS
    if random.random() > CROSSOVER_PROB:
        return list(parent1.chromosome), list(parent2.chromosome)
    child1_chromo, child2_chromo = [], []
    # Check if NUM_UAVS is valid
    if NUM_UAVS <= 0: return [], []
    for i in range(NUM_UAVS):
        if random.random() < 0.5:
            # Safe access to chromosome lists
            if i < len(parent1.chromosome): child1_chromo.append(parent1.chromosome[i])
            else: child1_chromo.append(0) # Default if index out of bounds (shouldn't happen)

            if i < len(parent2.chromosome): child2_chromo.append(parent2.chromosome[i])
            else: child2_chromo.append(0)
        else:
            if i < len(parent2.chromosome): child1_chromo.append(parent2.chromosome[i])
            else: child1_chromo.append(0)

            if i < len(parent1.chromosome): child2_chromo.append(parent1.chromosome[i])
            else: child2_chromo.append(0)
    return child1_chromo, child2_chromo

def mutate(chromosome):
    # (Keep the function from NSGA2.py - Bit Flip)
    global NUM_UAVS, MUTATION_PROB # Use global mutation prob
    mutated_chromosome = list(chromosome)
    if NUM_UAVS <= 0: return mutated_chromosome # Nothing to mutate
    for i in range(NUM_UAVS):
        # Ensure MUTATION_PROB is defined
        current_mutation_prob = MUTATION_PROB if 'MUTATION_PROB' in globals() else (1.5 / NUM_UAVS)
        if random.random() < current_mutation_prob:
            if i < len(mutated_chromosome): # Index safety
                 mutated_chromosome[i] = 1 - mutated_chromosome[i]
    return mutated_chromosome


# --- NSGA-II Visualization Functions ---
# (plot_nsga2_results remains mostly the same, as it plots objective values)
def plot_nsga2_results(feasible_pareto_front, representative_solutions, drone_nodes_original_data):
    # Keep this function as is from the previous version. It plots objective values stored
    # in the individuals, which are independent of how coverage was calculated.
    # Note: The title/labels related to coverage might need updating if the interpretation changed,
    # but the core plotting logic (parallel coords, scatter) is fine.
    if not PANDAS_AVAILABLE: print("\nNSGA-II Plotting skipped: pandas not installed."); return
    if not feasible_pareto_front: print("\nNSGA-II Plotting skipped: No feasible Pareto solutions found."); return

    print("\n--- Generating NSGA-II Plots ---")
    pareto_data_for_plot = []
    representative_indices_set = set(representative_solutions.values()) if representative_solutions else set()

    for i, ind in enumerate(feasible_pareto_front):
        if not ind.objectives or len(ind.objectives) < 4: continue
        avg_bat_plot = -ind.objectives[0]; avg_del_plot = ind.objectives[1]
        avg_uti_plot = ind.objectives[2]; num_sel_plot = ind.objectives[3]
        solution_type = "Other Pareto Solution"
        if i in representative_indices_set:
             labels = [label for label, idx in representative_solutions.items() if i == idx]
             if labels: solution_type = f"Focus: {labels[0]}"
        pareto_data_for_plot.append({
            'Avg Battery (%)': avg_bat_plot, 'Avg Delay (ms)': avg_del_plot,
            'Avg Utilization (%)': avg_uti_plot, 'Num UAVs': num_sel_plot,
            'Solution Type': solution_type })

    if not pareto_data_for_plot: print("No valid data points for NSGA-II plotting."); return
    pareto_df = pd.DataFrame(pareto_data_for_plot)
    color_map = { 'Focus: Best Avg Battery': '#e41a1c', 'Focus: Lowest Avg Delay': '#377eb8',
                  'Focus: Lowest Avg Utilization': '#4daf4a', 'Focus: Minimum UAVs': '#984ea3',
                  'Other Pareto Solution': 'darkgrey' }
    rep_types_ordered = list(color_map.keys())[:-1]

    # Parallel Coordinates Plot
    try:
        plt.figure(figsize=(15, 8))
        colors = [color_map.get(x, 'grey') for x in pareto_df['Solution Type']]
        linewidths = [2.5 if x != 'Other Pareto Solution' else 0.5 for x in pareto_df['Solution Type']]
        alphas = [1.0 if x != 'Other Pareto Solution' else 0.6 for x in pareto_df['Solution Type']]
        pc_plot = parallel_coordinates(
            pareto_df[['Avg Battery (%)', 'Avg Delay (ms)', 'Avg Utilization (%)', 'Num UAVs', 'Solution Type']],
            'Solution Type', color=colors, linewidth=linewidths, alpha=alphas )
        axes = plt.gca(); all_axes = plt.gcf().get_axes()
        numeric_axes = all_axes[:-1] if len(all_axes)>1 else [axes]
        if len(numeric_axes) >= 4:
            numeric_axes[0].invert_yaxis()
            numeric_axes[0].annotate("(Higher is Better)", xy=(0.5, 1.02), xycoords='axes fraction', ha='center', va='bottom', fontsize=9)
            for i in range(1, 4): numeric_axes[i].annotate("(Lower is Better)", xy=(0.5, 1.02), xycoords='axes fraction', ha='center', va='bottom', fontsize=9)
        plt.title('NSGA-II Pareto Solutions (Parallel Coordinates)', fontsize=14)
        axes.set_xticklabels(axes.get_xticklabels(), rotation=10, ha='right')
        plt.grid(True, axis='y', linestyle='--', alpha=0.6)
        legend_handles = [plt.Line2D([0], [0], color=color_map[label], lw=2.5) for label in rep_types_ordered]
        legend_handles.append(plt.Line2D([0], [0], color=color_map['Other Pareto Solution'], lw=0.5, alpha=0.6))
        legend_labels = rep_types_ordered + ['Other Pareto Solution']
        plt.legend(legend_handles, legend_labels, title="Solution Focus / Type", bbox_to_anchor=(1.02, 1), loc='upper left')
        plt.tight_layout(rect=[0, 0, 0.85, 1]); plt.show()
    except Exception as e: print(f"Error generating Parallel Coordinates Plot: {e}"); plt.close()

    # 2D Scatter Plots
    try:
        plt.figure(figsize=(18, 6.5))
        scatter_colors = [color_map.get(stype, 'grey') for stype in pareto_df['Solution Type']]
        scatter_sizes = [70 if stype != 'Other Pareto Solution' else 25 for stype in pareto_df['Solution Type']]
        scatter_alpha = [1.0 if stype != 'Other Pareto Solution' else 0.5 for stype in pareto_df['Solution Type']]
        plot_details = [ ('Avg Delay (ms)', 'Avg Battery (%)', "Delay vs Battery", "Avg Delay (ms) - Min", "Avg Battery (%) - Max"),
                         ('Avg Utilization (%)', 'Avg Battery (%)', "Util vs Battery", "Avg Util (%) - Min", "Avg Battery (%) - Max"),
                         ('Num UAVs', 'Avg Delay (ms)', "Num UAVs vs Delay", "Num Selected UAVs - Min", "Avg Delay (ms) - Min") ]
        for i, (x_col, y_col, title, x_lbl, y_lbl) in enumerate(plot_details, 1):
            plt.subplot(1, 3, i)
            plt.scatter(pareto_df[x_col], pareto_df[y_col], c=scatter_colors, s=scatter_sizes, alpha=scatter_alpha, edgecolors='w', linewidth=0.5)
            plt.xlabel(x_lbl); plt.ylabel(y_lbl); plt.title(title); plt.grid(True, linestyle='--', alpha=0.6)
        legend_handles_scatter = [plt.Line2D([0], [0], marker='o', color='w', markerfacecolor=color_map[label], markersize=8, markeredgecolor='k', lw=0) for label in rep_types_ordered]
        legend_handles_scatter.append(plt.Line2D([0], [0], marker='o', color='w', markerfacecolor=color_map['Other Pareto Solution'], markersize=6, alpha=0.7, markeredgecolor='k', lw=0))
        plt.figlegend(handles=legend_handles_scatter, labels=legend_labels, loc='lower center', ncol=len(legend_labels), bbox_to_anchor=(0.5, 0.01), title="Solution Focus / Type", fontsize='small')
        plt.suptitle('NSGA-II Pareto Front - Pairwise Objectives (Feasible)', y=0.99, fontsize=14)
        plt.tight_layout(rect=[0, 0.08, 1, 0.96]); plt.show()
    except Exception as e: print(f"Error generating Scatter Plots: {e}"); plt.close()



# =============================================================================
# SECTION 3: Main Execution Block
# =============================================================================

if __name__ == "__main__":
    # --- Shared Input Data ---
    master_gps = (34.03, -118.267) # Los Angeles Area Example
    drone_nodes_data = [ # Using the same data as before
        {'id': 'D01', 'gps': (34.043392, -118.266096), 'radius': 480.9, 'battery': 75, 'latency': 66, 'util': 37},
        {'id': 'D02', 'gps': (34.044353, -118.253013), 'radius': 399.4, 'battery': 47, 'latency': 293, 'util': 66},
        {'id': 'D03', 'gps': (34.035058, -118.247302), 'radius': 305.6, 'battery': 63, 'latency': 71, 'util': 37},
        {'id': 'D04', 'gps': (34.030712, -118.272819), 'radius': 489.6, 'battery': 68, 'latency': 121, 'util': 78},
        {'id': 'D05', 'gps': (34.050073, -118.273802), 'radius': 447.0, 'battery': 51, 'latency': 73, 'util': 17},
        {'id': 'D06', 'gps': (34.029068, -118.251203), 'radius': 497.8, 'battery': 44, 'latency': 180, 'util': 23},
        {'id': 'D07', 'gps': (34.026978, -118.293455), 'radius': 388.4, 'battery': 51, 'latency': 124, 'util': 72},
        {'id': 'D08', 'gps': (34.041605, -118.268935), 'radius': 388.5, 'battery': 50, 'latency': 279, 'util': 45},
        {'id': 'D09', 'gps': (34.046278, -118.261973), 'radius': 329.6, 'battery': 74, 'latency': 341, 'util': 40},
        {'id': 'D10', 'gps': (34.045880, -118.278790), 'radius': 482.7, 'battery': 51, 'latency': 63, 'util': 57},
        {'id': 'D11', 'gps': (34.049851, -118.261061), 'radius': 350.4, 'battery': 99, 'latency': 104, 'util': 39},
        {'id': 'D12', 'gps': (34.013688, -118.267234), 'radius': 463.4, 'battery': 81, 'latency': 228, 'util': 12},
        {'id': 'D13', 'gps': (34.033191, -118.253318), 'radius': 350.9, 'battery': 75, 'latency': 172, 'util': 61},
        {'id': 'D14', 'gps': (34.024652, -118.285789), 'radius': 352.1, 'battery': 30, 'latency': 148, 'util': 31},
        {'id': 'D15', 'gps': (34.035771, -118.261432), 'radius': 401.1, 'battery': 77, 'latency': 52, 'util': 20},
        {'id': 'D16', 'gps': (34.047357, -118.263858), 'radius': 346.6, 'battery': 34, 'latency': 165, 'util': 57},
        {'id': 'D17', 'gps': (34.022343, -118.248491), 'radius': 422.4, 'battery': 30, 'latency': 178, 'util': 14},
        {'id': 'D18', 'gps': (34.030328, -118.268348), 'radius': 390.9, 'battery': 42, 'latency': 184, 'util': 67},
        {'id': 'D19', 'gps': (34.033862, -118.286104), 'radius': 454.9, 'battery': 87, 'latency': 306, 'util': 36},
        {'id': 'D20', 'gps': (34.021212, -118.263919), 'radius': 454.7, 'battery': 76, 'latency': 170, 'util': 44},
        {'id': 'D21', 'gps': (34.038647, -118.259912), 'radius': 495.2, 'battery': 84, 'latency': 268, 'util': 36},
        {'id': 'D22', 'gps': (34.021705, -118.267649), 'radius': 435.8, 'battery': 80, 'latency': 70, 'util': 65},
        {'id': 'D23', 'gps': (34.025318, -118.289929), 'radius': 391.1, 'battery': 70, 'latency': 162, 'util': 32},
        {'id': 'D24', 'gps': (34.033464, -118.271027), 'radius': 354.7, 'battery': 69, 'latency': 136, 'util': 37},
        {'id': 'D25', 'gps': (34.019779, -118.244784), 'radius': 462.3, 'battery': 100, 'latency': 164, 'util': 58},
        {'id': 'D26', 'gps': (34.042328, -118.247414), 'radius': 300.5, 'battery': 48, 'latency': 278, 'util': 34},
        {'id': 'D27', 'gps': (34.040990, -118.285697), 'radius': 495.1, 'battery': 50, 'latency': 50, 'util': 66},
        {'id': 'D28', 'gps': (34.013872, -118.259444), 'radius': 340.9, 'battery': 66, 'latency': 277, 'util': 11},
        {'id': 'D29', 'gps': (34.027138, -118.274985), 'radius': 398.1, 'battery': 48, 'latency': 177, 'util': 47},
        {'id': 'D30', 'gps': (34.022039, -118.278626), 'radius': 371.8, 'battery': 32, 'latency': 144, 'util': 25},
        {'id': 'D31', 'gps': (34.036502, -118.262974), 'radius': 475.2, 'battery': 60, 'latency': 255, 'util': 50},
        {'id': 'D32', 'gps': (34.027050, -118.264546), 'radius': 307.2, 'battery': 54, 'latency': 134, 'util': 40},
        {'id': 'D33', 'gps': (34.026522, -118.264875), 'radius': 405.5, 'battery': 97, 'latency': 64, 'util': 58},
        {'id': 'D34', 'gps': (34.033895, -118.268872), 'radius': 457.7, 'battery': 97, 'latency': 69, 'util': 77},
        {'id': 'D35', 'gps': (34.027273, -118.263733), 'radius': 402.2, 'battery': 57, 'latency': 185, 'util': 62},
        {'id': 'D36', 'gps': (34.030269, -118.283818), 'radius': 405.9, 'battery': 98, 'latency': 274, 'util': 33},
        {'id': 'D37', 'gps': (34.030061, -118.265078), 'radius': 342.7, 'battery': 98, 'latency': 164, 'util': 75},
        {'id': 'D38', 'gps': (34.028212, -118.266964), 'radius': 308.1, 'battery': 40, 'latency': 250, 'util': 11},
        {'id': 'D39', 'gps': (34.021527, -118.288541), 'radius': 447.0, 'battery': 41, 'latency': 174, 'util': 56},
        {'id': 'D40', 'gps': (34.020084, -118.257594), 'radius': 454.1, 'battery': 87, 'latency': 252, 'util': 59},
        {'id': 'D41', 'gps': (34.044080, -118.281992), 'radius': 454.0, 'battery': 74, 'latency': 209, 'util': 61},
        {'id': 'D42', 'gps': (34.033277, -118.269163), 'radius': 387.4, 'battery': 94, 'latency': 337, 'util': 10},
        {'id': 'D43', 'gps': (34.010421, -118.262557), 'radius': 447.6, 'battery': 77, 'latency': 142, 'util': 62},
        {'id': 'D44', 'gps': (34.044962, -118.277139), 'radius': 474.9, 'battery': 30, 'latency': 91, 'util': 57},
        {'id': 'D45', 'gps': (34.034050, -118.272094), 'radius': 369.6, 'battery': 69, 'latency': 296, 'util': 42},
        {'id': 'D46', 'gps': (34.036090, -118.282563), 'radius': 456.5, 'battery': 81, 'latency': 195, 'util': 16},
        {'id': 'D47', 'gps': (34.027372, -118.260302), 'radius': 389.4, 'battery': 43, 'latency': 154, 'util': 57},
        {'id': 'D48', 'gps': (34.040514, -118.270071), 'radius': 499.0, 'battery': 67, 'latency': 67, 'util': 72},
        {'id': 'D49', 'gps': (34.033636, -118.283986), 'radius': 312.7, 'battery': 63, 'latency': 328, 'util': 31},
        {'id': 'D50', 'gps': (34.028736, -118.248488), 'radius': 369.8, 'battery': 36, 'latency': 314, 'util': 72}
    ]



    # --- Configuration ---
    greedy_task_type = 'sustain'
    greedy_target_coverage = 0.90
    greedy_grid_density = 40      # Density for area calculation
    show_greedy_plot = True

    # NSGA-II specific settings
    nsga2_population_size = 50
    nsga2_max_generations = 10
    nsga2_crossover_prob = 0.9
    # NSGA-II Coverage Target (Now relative to max possible area from Greedy)
    NSGA2_COVERAGE_TARGET = 0.90 # Set same as greedy target ratio
    # NSGA-II Grid settings REMOVED as coverage uses Greedy method

    print("Starting Combined Greedy + NSGA-II Workflow")
    print("INFO: NSGA-II will now use Greedy's coverage calculation method.")
    overall_start_time = time.time()

    # --- STEP 1: Run Greedy Workflow ---
    # This calculates the reference data needed for NSGA-II coverage
    greedy_results = run_greedy_workflow(
        master_node_gps=master_gps,
        drone_nodes_data=drone_nodes_data,
        task_type=greedy_task_type,
        target_coverage_ratio=greedy_target_coverage,
        grid_density=greedy_grid_density,
        show_plot=show_greedy_plot
    )

    # Extract greedy solution and crucial data for NSGA-II
    greedy_chromosome_solution = None
    if greedy_results and 'deployment_indicator' in greedy_results:
        greedy_chromosome_solution = greedy_results['deployment_indicator']
        # Populate global variables for NSGA-II functions
        ALL_GREED_NODES_GLOBAL = greedy_results.get('all_greed_nodes', [])
        PLOT_AREA_GLOBAL = greedy_results.get('plot_area_greedy', {})
        MAX_POSSIBLE_AREA_GLOBAL = greedy_results.get('max_possible_area', 0.0)
        GRID_DENSITY_GLOBAL = greedy_results.get('grid_density_used', DEFAULT_GRID_DENSITY_GREEDY)

        print("\nSuccessfully obtained Greedy solution and reference data for NSGA-II.")
        if MAX_POSSIBLE_AREA_GLOBAL <= 1e-6:
             print("WARNING: Max possible area from Greedy is near zero. NSGA-II coverage constraint might behave unexpectedly.")
        if not ALL_GREED_NODES_GLOBAL or not PLOT_AREA_GLOBAL:
             print("ERROR: Failed to get essential Greedy data (nodes/plot area) for NSGA-II. Exiting.")
             exit()
    else:
        print("\nError: Greedy workflow failed or did not return necessary data. Cannot proceed with NSGA-II. Exiting.")
        exit() # Cannot run NSGA-II without the reference data

    # --- STEP 2: Prepare Objective Data for NSGA-II ---
    # (Coverage data is already handled via globals from Greedy run)
    print("\n--- Preparing Objective Data for NSGA-II ---")
    NUM_UAVS = len(drone_nodes_data)
    if NUM_UAVS == 0: print("Error: No drone data. Exiting."); exit()
    if len(ALL_GREED_NODES_GLOBAL) != NUM_UAVS:
        print(f"Error: Mismatch between drone_nodes_data ({NUM_UAVS}) and all_greed_nodes ({len(ALL_GREED_NODES_GLOBAL)}). Exiting.")
        exit()

    # Set mutation probability
    MUTATION_PROB = 1.5 / NUM_UAVS if NUM_UAVS > 0 else 0.1
    print(f"NSGA-II Params: Pop={nsga2_population_size}, Gen={nsga2_max_generations}, Mut_Prob={MUTATION_PROB:.4f}")
    print(f"NSGA-II Target Cov Ratio (relative to max area): {NSGA2_COVERAGE_TARGET*100:.1f}%")

    # Initialize NSGA-II objective data structure
    UAV_DATA = {'battery': np.zeros(NUM_UAVS), 'delay': np.zeros(NUM_UAVS), 'utilization': np.zeros(NUM_UAVS)}
    processing_errors = False
    for i, drone in enumerate(drone_nodes_data):
        # Ensure order matches ALL_GREED_NODES_GLOBAL (which was based on drone_nodes_data order)
        if not all(k in drone for k in ['id', 'gps', 'radius', 'battery', 'latency', 'util']):
             print(f"Warning: Skipping drone {drone.get('id', i)} objective data due to missing fields.")
             UAV_DATA['battery'][i] = 0; UAV_DATA['delay'][i] = float('inf'); UAV_DATA['utilization'][i] = 100
             processing_errors = True
             continue
        UAV_DATA['battery'][i] = drone['battery']
        UAV_DATA['delay'][i] = drone['latency']
        UAV_DATA['utilization'][i] = drone['util']

    print(f"NSGA-II objective data processing complete for {NUM_UAVS} UAVs.")
    if processing_errors: print("Note: Warnings occurred during objective data processing.")


    # --- STEP 3: Initialize NSGA-II Population (P0) with Seeding ---
    print("\n--- Initializing NSGA-II Population (P0) ---")
    population = []

    # 1. Add the Greedy solution
    greedy_solution_added = False
    if greedy_chromosome_solution and len(greedy_chromosome_solution) == NUM_UAVS:
        print("Injecting Greedy solution into initial population...")
        try:
            # Create individual, repair (using new logic), and evaluate
            greedy_ind = Individual(chromosome=greedy_chromosome_solution)
            original_greedy_chromo_str = "".join(map(str, greedy_ind.chromosome))
            greedy_ind.chromosome = repair_individual(greedy_ind.chromosome) # Repair needed? Check feasibility first.
            repaired_greedy_chromo_str = "".join(map(str, greedy_ind.chromosome))
            if original_greedy_chromo_str != repaired_greedy_chromo_str:
                 print("  Note: Greedy chromosome was modified by NSGA-II repair function.")

            greedy_ind.calculate_fitness() # Calculates objectives and feasibility (using new check_constraints)
            population.append(greedy_ind)
            greedy_solution_added = True
            print(f"Greedy Individual added. Feasible for NSGA-II: {greedy_ind.is_feasible}, "
                  f"Objectives: [-Bat={greedy_ind.objectives[0]:.2f}, Delay={greedy_ind.objectives[1]:.2f}, Util={greedy_ind.objectives[2]:.2f}, Num={greedy_ind.objectives[3]}]")
            if not greedy_ind.is_feasible:
                 # This might happen if NSGA2_COVERAGE_TARGET is higher than greedy_target_coverage
                 print("  Warning: The provided Greedy solution (even after repair) is not feasible under NSGA-II constraints/target.")
        except Exception as e:
            print(f"Error creating/evaluating Individual from Greedy chromosome: {e}")
            print("Skipping Greedy solution injection.")
    # (Rest of initialization remains the same, using the new Individual constructor)
    num_to_generate = nsga2_population_size - len(population)
    print(f"Generating {num_to_generate} additional random individuals for P0...")
    init_attempts = 0
    max_init_attempts = num_to_generate * 5
    while len(population) < nsga2_population_size and init_attempts < max_init_attempts:
         try:
             ind = Individual() # Uses new repair/check internally
             ind.calculate_fitness()
             population.append(ind)
             init_attempts += 1
         except Exception as e:
              print(f"Error generating random individual {len(population)}: {e}")
              init_attempts += 1
              if init_attempts >= max_init_attempts: break

    print(f"Initial population P0 created with {len(population)} individuals.")
    if len(population) < nsga2_population_size: print(f"Warning: Could not generate full initial population size.")
    if not population: print("FATAL ERROR: No individuals in initial population. Exiting."); exit()

    # --- STEP 4: Run NSGA-II Evolution Loop ---
    # (The loop structure remains the same. It uses the modified Individual,
    # selection, crossover, mutation, repair, and constraint checking functions)
    print("\n--- Starting NSGA-II Evolution Loop ---")
    nsga_start_time = time.time()
    fronts_indices_p0 = fast_non_dominated_sort(population)
    for front_idx_list in fronts_indices_p0:
        calculate_crowding_distance(front_idx_list, population)

    for generation in range(nsga2_max_generations):
        gen_start_time = time.time()
        offspring_population = []
        while len(offspring_population) < nsga2_population_size:
            try:
                parent1 = binary_tournament_selection(population)
                parent2 = binary_tournament_selection(population)
                child1_chromo, child2_chromo = crossover(parent1, parent2)
                child1_chromo = mutate(child1_chromo)
                child2_chromo = mutate(child2_chromo)
                child1_chromo = repair_individual(child1_chromo) # Uses new repair
                child2_chromo = repair_individual(child2_chromo) # Uses new repair
                child1 = Individual(chromosome=child1_chromo); child1.calculate_fitness()
                offspring_population.append(child1)
                if len(offspring_population) < nsga2_population_size:
                    child2 = Individual(chromosome=child2_chromo); child2.calculate_fitness()
                    offspring_population.append(child2)
            except Exception as e:
                print(f"Error during offspring generation (Gen {generation+1}): {e}")
                if len(offspring_population) < nsga2_population_size // 2: break
                else: continue
        if len(offspring_population) < nsga2_population_size:
             print(f"Warning: Stopping generation {generation+1} early due to offspring errors.")
             # Decide whether to continue with fewer offspring or stop
             if len(offspring_population) == 0 : break # Stop if no offspring

        combined_population = population + offspring_population
        fronts_indices = fast_non_dominated_sort(combined_population)
        next_population_indices = []
        front_index = 0
        while front_index < len(fronts_indices) and \
              len(next_population_indices) + len(fronts_indices[front_index]) <= nsga2_population_size:
            calculate_crowding_distance(fronts_indices[front_index], combined_population)
            next_population_indices.extend(fronts_indices[front_index])
            front_index += 1
        if front_index < len(fronts_indices) and len(next_population_indices) < nsga2_population_size:
            critical_front_indices = fronts_indices[front_index]
            calculate_crowding_distance(critical_front_indices, combined_population)
            critical_front_indices.sort(key=lambda i: combined_population[i].crowding_distance, reverse=True)
            remaining_spots = nsga2_population_size - len(next_population_indices)
            next_population_indices.extend(critical_front_indices[:remaining_spots])
        population = [combined_population[i] for i in next_population_indices]
        gen_end_time = time.time()
        if (generation + 1) % 10 == 0 or generation == 0 or generation == nsga2_max_generations - 1:
             num_feasible = sum(1 for ind in population if ind.is_feasible) # is_feasible uses new check
             print(f"Gen {generation+1:>{len(str(nsga2_max_generations))}}/{nsga2_max_generations} | Feasible: {num_feasible}/{len(population)} | Time: {gen_end_time - gen_start_time:.2f}s")
             first_front_new = [ind for ind in population if ind.rank == 0]
             if first_front_new:
                  num_feasible_f0 = sum(1 for ind in first_front_new if ind.is_feasible)
                  print(f"  Pareto Front (Rank 0) Size: {len(first_front_new)} ({num_feasible_f0} feasible)")

    nsga_end_time = time.time()
    print(f"\n--- NSGA-II Evolution Finished ---")
    print(f"NSGA-II execution time: {nsga_end_time - nsga_start_time:.2f} seconds.")


    # --- STEP 5: Process and Display NSGA-II Results ---
    print("\n--- Processing Final NSGA-II Results ---")
    final_fronts_indices = fast_non_dominated_sort(population)
    if not final_fronts_indices or not final_fronts_indices[0]:
         print("Error: No Pareto front (Rank 0) found in final population.")
         feasible_pareto_front = []
    else:
         final_pareto_indices = final_fronts_indices[0]
         for idx in final_pareto_indices: # Ensure fitness/feasibility calculated with final state
              if not population[idx].objectives: population[idx].calculate_fitness()
         feasible_pareto_front = [population[i] for i in final_pareto_indices if population[i].is_feasible] # is_feasible uses new check

    if not feasible_pareto_front:
        print("No feasible non-dominated solutions found in the final Pareto front.")
        # (Optional: Check if greedy solution survived, etc.)
        if any(ind.is_feasible for ind in population): print(f"  However, {sum(1 for ind in population if ind.is_feasible)} feasible solutions exist (dominated).")
        else: print("  No feasible solutions found in the entire final population.")
    else:
        print(f"Found {len(feasible_pareto_front)} feasible Pareto optimal solutions.")
        feasible_pareto_front.sort(key=lambda x: x.objectives[3] if x.objectives and len(x.objectives)>3 else float('inf'))

        # Select Representative Solutions (logic remains the same)
        representative_solutions = {}
        if feasible_pareto_front:
             # Ensure objectives exist before finding min/max
             valid_for_bat = [i for i, ind in enumerate(feasible_pareto_front) if ind.objectives and len(ind.objectives)>0]
             valid_for_del = [i for i, ind in enumerate(feasible_pareto_front) if ind.objectives and len(ind.objectives)>1]
             valid_for_uti = [i for i, ind in enumerate(feasible_pareto_front) if ind.objectives and len(ind.objectives)>2]
             valid_for_num = [i for i, ind in enumerate(feasible_pareto_front) if ind.objectives and len(ind.objectives)>3]

             if valid_for_bat: representative_solutions["Best Avg Battery"] = min(valid_for_bat, key=lambda i: feasible_pareto_front[i].objectives[0])
             if valid_for_del: representative_solutions["Lowest Avg Delay"] = min(valid_for_del, key=lambda i: feasible_pareto_front[i].objectives[1])
             if valid_for_uti: representative_solutions["Lowest Avg Utilization"] = min(valid_for_uti, key=lambda i: feasible_pareto_front[i].objectives[2])
             if valid_for_num: representative_solutions["Minimum UAVs"] = min(valid_for_num, key=lambda i: feasible_pareto_front[i].objectives[3]) # First one due to sort

        # Print Representative Solutions
        print("\n--- Four Representative Feasible Pareto Solutions ---")
        printed_indices = set()
        option_count = 0
        if representative_solutions:
             for label, idx in representative_solutions.items():
                  if idx < len(feasible_pareto_front) and idx not in printed_indices:
                      option_count += 1
                      ind = feasible_pareto_front[idx]
                      if not ind.objectives or len(ind.objectives) < 4: continue # Safety check

                      avg_bat = -ind.objectives[0]; avg_del = ind.objectives[1]; avg_uti = ind.objectives[2]; num_sel = int(ind.objectives[3])
                      is_feas = ind.is_feasible # Feasibility already checked

                      # *** Calculate Coverage using Greedy Method for Reporting ***
                      selected_indices_report = [j for j, sel in enumerate(ind.chromosome) if sel == 1]
                      selected_nodes_report = [ALL_GREED_NODES_GLOBAL[j] for j in selected_indices_report if 0 <= j < len(ALL_GREED_NODES_GLOBAL)]
                      coverage_area = calculate_union_area(selected_nodes_report, PLOT_AREA_GLOBAL, GRID_DENSITY_GLOBAL)
                      coverage_ratio = (coverage_area / MAX_POSSIBLE_AREA_GLOBAL * 100) if MAX_POSSIBLE_AREA_GLOBAL > 1e-6 else (100 if coverage_area > 1e-6 else 0)

                      print(f"Option {option_count}: Focus on {label} (Solution Index {idx})")
                      print(f"  Avg Battery: {avg_bat:.2f}%")
                      print(f"  Avg Delay: {avg_del:.2f} ms")
                      print(f"  Avg Utilization: {avg_uti:.2f}%")
                      print(f"  Num Selected UAVs: {num_sel}")
                      # Report coverage based on the consistent Greedy method
                      print(f"  Coverage Area (Est): {coverage_area:.2f} m²")
                      print(f"  Coverage Ratio (Est): {coverage_ratio:.2f}% (Target: {NSGA2_COVERAGE_TARGET*100:.1f}%, Met: {is_feas})")

                      selected_ids_indices = [j for j, sel in enumerate(ind.chromosome) if sel == 1]
                      selected_drone_input_ids = [drone_nodes_data[j]['id'] for j in selected_ids_indices if j < len(drone_nodes_data) and 'id' in drone_nodes_data[j]]
                      print(f"  Selected UAV IDs: {', '.join(selected_drone_input_ids)}")
                      print("-" * 30)
                      printed_indices.add(idx)
        else: print("Could not determine representative solutions.")

        # --- STEP 6: Visualize NSGA-II Results ---
        # Plotting function remains the same as it visualizes objectives
        plot_nsga2_results(feasible_pareto_front, representative_solutions, drone_nodes_data)

    overall_end_time = time.time()
    print("\nCombined Workflow Finished.")
    print(f"Total execution time: {overall_end_time - overall_start_time:.2f} seconds.")