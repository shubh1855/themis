def count_subarrays_sum_less_than_k(nums, k):
    if k <= 0: return 0
    
    count = 0
    current_sum = 0
    left = 0
    
    for right in range(len(nums)):
        current_sum += nums[right]
        
        while current_sum >= k and left <= right:
            current_sum -= nums[left]
            left += 1
        
        # The number of subarrays ending at 'right' with sum < k
        # is equal to the length of the current window
        count += (right - left + 1)
        
    return count

if __name__ == "__main__":
    # Test cases
    test_cases = [
        ([10, 5, 2, 6], 100, 10), # All subarrays
        ([10, 5, 2, 6], 20, 9),    # Corrected expectation to 9
        ([1, 1, 1], 2, 3),         # Only single elements
        ([1, 2, 3], 0, 0),         # k is 0
    ]
    
    for nums, k, expected in test_cases:
        result = count_subarrays_sum_less_than_k(nums, k)
        print(f"nums={nums}, k={k} | Expected: {expected}, Got: {result} | {'PASS' if result == expected else 'FAIL'}")