document.addEventListener('DOMContentLoaded', () => {
    const form = document.getElementById('generate-form');
    const imageUpload = document.getElementById('image-upload');
    const preview = document.getElementById('preview');
    const imagePreviewContainer = document.getElementById('image-preview-container');
    const stepsInput = document.getElementById('steps');
    const stepsValue = document.getElementById('steps-value');
    const resultContainer = document.getElementById('result-container');
    const loading = document.getElementById('loading');
    const submitBtn = document.getElementById('submit-btn');
    const optimizeBtn = document.getElementById('optimize-btn');
    const promptInput = document.getElementById('prompt');
    const clearImageBtn = document.getElementById('clear-image-btn');
    const modelSelect = document.getElementById('model');
    const imageUploadGroup = document.getElementById('image-upload-group');
   
    // Handle prompt optimization
    optimizeBtn.addEventListener('click', async () => {
    	const originalPrompt = promptInput.value;
    	if (!originalPrompt) {
    		alert('请输入提示词后再进行优化。');
    		return;
    	}
   
    	optimizeBtn.disabled = true;
    	optimizeBtn.textContent = '优化中...';
   
    	try {
    		const response = await fetch('/api/optimize-prompt', {
    			method: 'POST',
    			headers: {
    				'Content-Type': 'application/json',
    			},
    			body: JSON.stringify({ prompt: originalPrompt }),
    		});
   
    		if (!response.ok) {
    			throw new Error('优化提示词失败');
    		}
   
    		const data = await response.json();
    		if (data.success && data.optimizedPrompt) {
    			promptInput.value = data.optimizedPrompt;
    		} else {
    			throw new Error('无效的优化结果');
    		}
   
    	} catch (error) {
    		console.error('Error optimizing prompt:', error);
    		alert(`优化失败: ${error.message}`);
    	} finally {
    		optimizeBtn.disabled = false;
    		optimizeBtn.textContent = '优化提示词';
    	}
    });

    // --- System-Wide Image Handling Logic (Refactored for Robustness) ---

    // 1. Central function to update the entire preview UI state based on file input.
    // This is the single source of truth for the preview UI.
    const updateImagePreview = () => {
        const file = imageUpload.files[0];
        if (file) {
            // A file is selected: show preview and clear button.
            const reader = new FileReader();
            reader.onload = (e) => {
                preview.src = e.target.result;
                imagePreviewContainer.style.display = 'flex';
                clearImageBtn.classList.remove('hidden');
            };
            reader.readAsDataURL(file);
        } else {
            // No file is selected: hide preview and clear button.
            preview.src = '#';
            imagePreviewContainer.style.display = 'none';
            clearImageBtn.classList.add('hidden');
        }
    };

    // 2. Function to show/hide the entire upload group based on the selected model.
    const toggleImageUploadGroup = () => {
        const selectedModel = modelSelect.value;
        const imageUploadModels = ['Flux-Kontext', 'Qwen-Image-Edit'];

        if (imageUploadModels.includes(selectedModel)) {
            imageUploadGroup.classList.remove('hidden');
            imageUpload.required = true;
        } else {
            imageUploadGroup.classList.add('hidden');
            imageUpload.required = false;
            // If the group is hidden, ensure the file input is cleared and the preview is hidden.
            if (imageUpload.value) {
                imageUpload.value = '';
                updateImagePreview();
            }
        }
    };

    // 3. Event Listeners Setup
    
    // When model is changed, toggle the entire group's visibility.
    modelSelect.addEventListener('change', toggleImageUploadGroup);

    // When a file is selected (or selection is cancelled), update the preview UI.
    imageUpload.addEventListener('change', updateImagePreview);

    // When clear button is clicked, clear the file input and then update the preview UI.
    clearImageBtn.addEventListener('click', () => {
        imageUpload.value = '';
        updateImagePreview();
    });

    // 4. Initial State Setup on Page Load
    toggleImageUploadGroup();
    updateImagePreview(); // Ensure preview is hidden initially.
    
    // --- End of Image Handling Logic ---
   
    // Update steps value display
    stepsInput.addEventListener('input', () => {
    	stepsValue.textContent = stepsInput.value;
    });

    // Handle form submission
    form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const formData = new FormData(form);
        
        // Show loading indicator
        loading.classList.remove('hidden');
        resultContainer.innerHTML = '<p>在这里查看您生成的图片。</p>';
        submitBtn.disabled = true;
        submitBtn.textContent = '生成中...';

        try {
            const response = await fetch('/api/generate', {
                method: 'POST',
                body: formData,
            });

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.error || '生成图片时发生未知错误');
            }

            const blob = await response.blob();
            const imageUrl = URL.createObjectURL(blob);
            
            resultContainer.innerHTML = `<img src="${imageUrl}" alt="生成结果">`;

        } catch (error) {
            console.error('Error:', error);
            resultContainer.innerHTML = `<p style="color: red;">错误: ${error.message}</p>`;
        } finally {
            // Hide loading indicator
            loading.classList.add('hidden');
            submitBtn.disabled = false;
            submitBtn.textContent = '生成图片';
        }
    });
});